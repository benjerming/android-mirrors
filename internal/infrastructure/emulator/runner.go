// Package emulator 提供 Android 模拟器进程的真实启动与停止能力。
//
// 它包装了 emulator 和 adb 两个命令行工具，让上层应用服务无需关心具体的端口分配、
// boot 等待、kill 命令等基础设施细节。
package emulator

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Runner 表示真实的模拟器运行器，负责启动 emulator 子进程并通过 adb 探测设备就绪。
type Runner struct {
	emulatorBin string                  // emulatorBin 表示 emulator 可执行文件路径。
	adbBin      string                  // adbBin 表示 adb 可执行文件路径。
	logDir      string                  // logDir 表示 emulator stdout/stderr 落盘目录；为空时退回 data/logs/emulator。
	logger      *slog.Logger            // logger 用来打印底层启停过程，nil 时退回 slog.Default。
	mu          sync.Mutex              // mu 保护 procs / reserved，避免并发启动同一实例时打架。
	procs       map[string]*runningProc // procs 记录每个实例对应的子进程，停止时回收用。
	reserved    map[int]struct{}        // reserved 是 allocatePort 在 adb 看到设备前的进程内占位，避免并发 Start 撞同一端口。
}

// runningProc 表示一个正在运行的模拟器子进程及其分配到的 adb 序列号。
type runningProc struct {
	cmd     *exec.Cmd
	serial  string
	logFile *os.File // logFile 是该 emulator 进程 stdout/stderr 的输出文件，Stop 时关闭。
}

// NewRunner 用来按 ANDROID_HOME / PATH 自动定位 emulator 和 adb。
func NewRunner() *Runner {
	r := &Runner{
		emulatorBin: locateBin("emulator", "emulator", "emulator"),
		adbBin:      locateBin("adb", "platform-tools", "adb"),
		procs:       map[string]*runningProc{},
		reserved:    map[int]struct{}{},
	}
	r.log().Info("emulator runner initialized", "emulator_bin", r.emulatorBin, "adb_bin", r.adbBin)
	return r
}

// WithLogger 注入运行日志使用的 logger，未注入时使用 slog.Default()。
func (r *Runner) WithLogger(l *slog.Logger) *Runner {
	r.logger = l
	return r
}

// WithLogDir 注入 emulator stdout/stderr 的落盘目录。每个实例一个文件
// （emulator-<name>.log），让 Go 主日志保持干净；boot 失败时尾部 4 KB 会被回吐到 slog.Error。
// 未注入时退回 data/logs/emulator/。
func (r *Runner) WithLogDir(dir string) *Runner {
	r.logDir = dir
	return r
}

// resolveLogDir 返回最终落盘目录，并按需创建。
func (r *Runner) resolveLogDir() (string, error) {
	dir := r.logDir
	if dir == "" {
		dir = filepath.Join("data", "logs", "emulator")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// tailFile 读取文件尾部最多 n 字节，给"启动失败时把现场附在 slog.Error 里"用。
func tailFile(path string, n int64) string {
	f, err := os.Open(path) // #nosec G304 -- 路径来自 resolveLogDir + 实例名，受控。
	if err != nil {
		return ""
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return ""
	}
	size := stat.Size()
	if size <= 0 {
		return ""
	}
	if size > n {
		if _, err := f.Seek(-n, 2); err != nil {
			return ""
		}
	}
	buf := make([]byte, n)
	read, _ := f.Read(buf)
	return string(buf[:read])
}

func (r *Runner) log() *slog.Logger {
	if r.logger != nil {
		return r.logger
	}
	return slog.Default()
}

// Start 用来在后台启动指定 AVD 的模拟器进程，并返回 adb 序列号（如 "emulator-5554"）。
//
// 步骤大致是：
//  1. 选一个未被占用的偶数端口（5554、5556 等都是 emulator 的常用约定）。
//  2. 启动 emulator -avd <name> -port <port> -no-window -gpu host，让它跑在后台。
//  3. 通过 adb 等待设备进入 boot completed 状态。
func (r *Runner) Start(ctx context.Context, instanceName string) (string, error) {
	log := r.log().With("instance", instanceName)
	log.InfoContext(ctx, "runner.Start invoked")

	if r.emulatorBin == "" || r.adbBin == "" {
		log.ErrorContext(ctx, "emulator/adb binary not found", "emulator_bin", r.emulatorBin, "adb_bin", r.adbBin)
		return "", fmt.Errorf("emulator/adb not found: set ANDROID_HOME or ANDROID_SDK_ROOT")
	}

	r.mu.Lock()
	if existing, ok := r.procs[instanceName]; ok {
		r.mu.Unlock()
		log.InfoContext(ctx, "instance already running, returning existing serial", "serial", existing.serial)
		return existing.serial, nil
	}
	r.mu.Unlock()

	port, err := r.reservePort(ctx)
	if err != nil {
		log.ErrorContext(ctx, "port allocation failed", "error", err)
		return "", err
	}
	serial := fmt.Sprintf("emulator-%d", port)
	log = log.With("port", port, "serial", serial)
	log.InfoContext(ctx, "port reserved")

	// args := []string{"-avd", instanceName, "-port", fmt.Sprintf("%d", port), "-no-snapshot", "-no-audio", "-gpu", "host", "-no-qt"}
	args := []string{"-avd", instanceName, "-port", fmt.Sprintf("%d", port), "-no-snapshot", "-no-audio"}
	log.InfoContext(ctx, "spawning emulator process", "bin", r.emulatorBin, "args", strings.Join(args, " "))

	logDir, err := r.resolveLogDir()
	if err != nil {
		r.releasePort(port)
		log.ErrorContext(ctx, "create emulator log dir failed", "error", err)
		return "", fmt.Errorf("emulator log dir: %w", err)
	}
	logPath := filepath.Join(logDir, fmt.Sprintf("emulator-%s.log", instanceName))
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644) // #nosec G304 -- 路径来自 resolveLogDir + 实例名，受控。
	if err != nil {
		r.releasePort(port)
		log.ErrorContext(ctx, "open emulator log file failed", "error", err, "path", logPath)
		return "", fmt.Errorf("open emulator log: %w", err)
	}

	cmd := exec.Command(r.emulatorBin, args...) // #nosec G204 -- 参数来自模板/内部分配，可信
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		r.releasePort(port)
		log.ErrorContext(ctx, "failed to spawn emulator process", "error", err)
		return "", fmt.Errorf("start emulator %s: %w", instanceName, err)
	}
	log.InfoContext(ctx, "emulator process spawned", "pid", cmd.Process.Pid, "log", logPath)

	if err := r.waitForBoot(ctx, serial); err != nil {
		log.ErrorContext(ctx, "boot wait failed, killing process",
			"error", err, "pid", cmd.Process.Pid, "log", logPath, "tail", tailFile(logPath, 4096))
		_ = cmd.Process.Kill()
		_ = logFile.Close()
		r.releasePort(port)
		return "", fmt.Errorf("wait emulator %s boot: %w", instanceName, err)
	}
	log.InfoContext(ctx, "emulator boot completed")

	r.mu.Lock()
	r.procs[instanceName] = &runningProc{cmd: cmd, serial: serial, logFile: logFile}
	r.mu.Unlock()
	return serial, nil
}

// Stop 用来通过 adb 关掉某个序列号对应的模拟器，并回收子进程登记表。
func (r *Runner) Stop(ctx context.Context, serial string) error {
	log := r.log().With("serial", serial)
	log.InfoContext(ctx, "runner.Stop invoked")

	if strings.TrimSpace(serial) == "" {
		log.InfoContext(ctx, "empty serial, nothing to stop")
		return nil
	}
	if r.adbBin == "" {
		log.ErrorContext(ctx, "adb binary not found")
		return fmt.Errorf("adb not found: set ANDROID_HOME or ANDROID_SDK_ROOT")
	}

	cmd := exec.CommandContext(ctx, r.adbBin, "-s", serial, "emu", "kill") // #nosec G204
	if err := cmd.Run(); err != nil {
		// kill 失败不一定致命：可能进程已经自己挂了，这里只记录但不阻塞回收。
		log.WarnContext(ctx, "adb emu kill returned error (process may already be gone)", "error", err)
	} else {
		log.InfoContext(ctx, "adb emu kill sent")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	for name, proc := range r.procs {
		if proc.serial == serial {
			// 给 emulator 一点时间自己退出；超时再硬杀。
			done := make(chan struct{})
			go func() { _, _ = proc.cmd.Process.Wait(); close(done) }()
			select {
			case <-done:
				log.InfoContext(ctx, "emulator process exited cleanly", "instance", name)
			case <-time.After(5 * time.Second):
				log.WarnContext(ctx, "emulator did not exit in 5s, force killing", "instance", name, "pid", proc.cmd.Process.Pid)
				_ = proc.cmd.Process.Kill()
			}
			if proc.logFile != nil {
				_ = proc.logFile.Close()
			}
			delete(r.procs, name)
			if port := portFromSerial(serial); port > 0 {
				delete(r.reserved, port)
			}
			break
		}
	}
	return nil
}

// reservePort 在锁内一次性查 adb + 进程内占位，避免并发 Start 拿到同一端口。
//
// 之前的实现只看 adb devices，导致并发启动 N 台时所有 goroutine 同时看到"5554 空闲"，
// 都用 5554 起 emulator，结果只有一台真的能起来——这是用户反馈"4 台只见 1 台"的根因。
func (r *Runner) reservePort(ctx context.Context) (int, error) {
	used, err := r.usedPorts(ctx)
	if err != nil {
		return 0, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for p := range r.reserved {
		used[p] = struct{}{}
	}
	for port := 5554; port <= 5682; port += 2 {
		if _, taken := used[port]; !taken {
			r.reserved[port] = struct{}{}
			r.log().DebugContext(ctx, "reservePort picked", "port", port, "adb_used", len(used)-len(r.reserved)+1, "in_process_reserved", len(r.reserved))
			return port, nil
		}
	}
	return 0, errors.New("no free emulator port in 5554-5682")
}

// releasePort 在 Start 失败路径上把进程内占位还回去，避免 5554-5682 被失败启动占满。
func (r *Runner) releasePort(port int) {
	r.mu.Lock()
	delete(r.reserved, port)
	r.mu.Unlock()
}

// portFromSerial 从 "emulator-5554" 解析端口号；解析失败返回 0。
func portFromSerial(serial string) int {
	var port int
	if _, err := fmt.Sscanf(serial, "emulator-%d", &port); err != nil {
		return 0
	}
	return port
}

// usedPorts 用来从 adb devices 输出里解析出已经被占用的 emulator 端口。
func (r *Runner) usedPorts(ctx context.Context) (map[int]struct{}, error) {
	cmd := exec.CommandContext(ctx, r.adbBin, "devices") // #nosec G204
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("adb devices: %w", err)
	}
	used := map[int]struct{}{}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 1 || !strings.HasPrefix(fields[0], "emulator-") {
			continue
		}
		var port int
		if _, err := fmt.Sscanf(fields[0], "emulator-%d", &port); err == nil {
			used[port] = struct{}{}
		}
	}
	return used, nil
}

// waitForBoot 用来轮询 adb，直到目标 serial 报告 sys.boot_completed = 1。
//
// 这里给到 3 分钟超时——冷启动一个全新 AVD 通常 1 分钟左右，留点余量。
func (r *Runner) waitForBoot(ctx context.Context, serial string) error {
	log := r.log().With("serial", serial)
	deadline := time.Now().Add(3 * time.Minute)
	start := time.Now()
	attempts := 0
	log.InfoContext(ctx, "waiting for boot completed", "deadline_seconds", 180)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			log.WarnContext(ctx, "boot wait cancelled by context", "error", ctx.Err())
			return ctx.Err()
		default:
		}

		attempts++
		cmd := exec.CommandContext(ctx, r.adbBin, "-s", serial, "shell", "getprop", "sys.boot_completed") // #nosec G204
		out, err := cmd.Output()
		trimmed := strings.TrimSpace(string(out))
		if err == nil && trimmed == "1" {
			log.InfoContext(ctx, "boot completed", "elapsed_seconds", int(time.Since(start).Seconds()), "attempts", attempts)
			return nil
		}
		// 每 ~20s（每 10 次轮询）打一条 debug，避免日志过吵但仍能看到进度。
		if attempts%10 == 0 {
			log.DebugContext(ctx, "still waiting for boot", "elapsed_seconds", int(time.Since(start).Seconds()), "attempts", attempts, "last_output", trimmed, "last_error", errString(err))
		}
		time.Sleep(2 * time.Second)
	}
	log.ErrorContext(ctx, "boot wait timed out", "elapsed_seconds", int(time.Since(start).Seconds()), "attempts", attempts)
	return errors.New("timeout waiting for boot")
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// locateBin 用来按 ANDROID_HOME 子目录 → PATH 顺序定位某个 SDK 工具。
func locateBin(name, sdkDir, file string) string {
	for _, root := range []string{os.Getenv("ANDROID_HOME"), os.Getenv("ANDROID_SDK_ROOT")} {
		if root == "" {
			continue
		}
		candidate := filepath.Join(root, sdkDir, file)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	return ""
}
