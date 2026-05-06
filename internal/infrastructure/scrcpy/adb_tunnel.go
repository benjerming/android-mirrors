package scrcpy

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrAdbUnavailable 表示 adb 二进制未配置。
var ErrAdbUnavailable = errors.New("scrcpy: adb binary not configured")

// CommandRunner 抽象命令执行器，便于注入 fake。
type CommandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

func defaultRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// AdbTunnel 是 scrcpy 集成专用的 adb 命令拼装器。不复用
// internal/infrastructure/adb，因为后者只覆盖 input/install 等业务命令，
// 把 reverse/shell-exec 揉进去会破坏其单一职责。
type AdbTunnel struct {
	binary string
	runner CommandRunner
	// starter 用于启动长生命周期的 server 进程；测试可注入 fake。
	starter func(ctx context.Context, name string, args ...string) (*exec.Cmd, error)
}

func NewAdbTunnel(binary string) *AdbTunnel {
	return &AdbTunnel{
		binary:  strings.TrimSpace(binary),
		runner:  defaultRunner,
		starter: defaultStarter,
	}
}

func defaultStarter(ctx context.Context, name string, args ...string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func (t *AdbTunnel) WithRunner(r CommandRunner) *AdbTunnel {
	t.runner = r
	// 当注入 runner（典型 = 单测）时，starter 也走 runner，让单测能断言 StartServer 的 args。
	t.starter = func(ctx context.Context, name string, args ...string) (*exec.Cmd, error) {
		if _, err := r(ctx, name, args...); err != nil {
			return nil, err
		}
		return exec.Command("true"), nil
	}
	return t
}

func (t *AdbTunnel) run(ctx context.Context, args ...string) ([]byte, error) {
	if t.binary == "" {
		return nil, ErrAdbUnavailable
	}
	return t.runner(ctx, t.binary, args...)
}

// Push 执行 `adb -s <serial> push <local> <remote>`。
func (t *AdbTunnel) Push(ctx context.Context, serial, local, remote string) error {
	_, err := t.run(ctx, "-s", serial, "push", local, remote)
	return err
}

// Reverse 执行 `adb -s <serial> reverse localabstract:<name> tcp:<port>`。
//
// 设备侧的 abstract socket 名按 device 隔离，多模拟器并发不会撞名；
// 服务端只需要一个 ephemeral 端口。
func (t *AdbTunnel) Reverse(ctx context.Context, serial, abstractName string, tcpPort int) error {
	_, err := t.run(ctx, "-s", serial, "reverse",
		"localabstract:"+abstractName, fmt.Sprintf("tcp:%d", tcpPort))
	return err
}

// RemoveReverse 撤销之前 Reverse 注册的映射；session 关闭时调用。
func (t *AdbTunnel) RemoveReverse(ctx context.Context, serial, abstractName string) error {
	_, err := t.run(ctx, "-s", serial, "reverse", "--remove", "localabstract:"+abstractName)
	return err
}

// ServerStartOpts 是启动 scrcpy-server 的关键参数。
type ServerStartOpts struct {
	Version    string
	BitrateBps int
	MaxFps     int
	MaxSize    int // 0 = 不限制
}

// StartServer 不等待进程退出；返回的 *exec.Cmd 由调用方保管，
// 在 session 结束时 Process.Kill + Wait。
//
// 调用前提：adb reverse 已注册。scrcpy-server 启动后会主动 connect
// 回服务端 listener（tunnel_forward=false 模式）。
func (t *AdbTunnel) StartServer(ctx context.Context, serial string, opts ServerStartOpts) (*exec.Cmd, error) {
	if t.binary == "" {
		return nil, ErrAdbUnavailable
	}
	args := []string{
		"-s", serial, "shell",
		"CLASSPATH=/data/local/tmp/scrcpy-server.jar",
		"app_process", "/", "com.genymobile.scrcpy.Server",
		opts.Version,
		"tunnel_forward=false",
		"audio=false",
		"control=true",
		// 关掉剪贴板自动同步：默认开启时 scrcpy 监听设备前台 app/IME 变化、
		// 主动 setClipboard 把主机剪贴板推到设备 → 设备弹"shell pasted from your clipboard"。
		// 群控场景下不需要这个功能；关掉避免每次切窗口都弹一次。
		"clipboard_autosync=false",
		fmt.Sprintf("video_bit_rate=%d", opts.BitrateBps),
		fmt.Sprintf("max_fps=%d", opts.MaxFps),
	}
	if opts.MaxSize > 0 {
		args = append(args, fmt.Sprintf("max_size=%d", opts.MaxSize))
	}
	return t.starter(ctx, t.binary, args...)
}
