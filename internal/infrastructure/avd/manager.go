// Package avd 提供 Android 虚拟设备（AVD）的真实创建与删除能力。
//
// 这一层把"调用 Android SDK 命令行工具"这件事封装起来，让上层应用服务不必关心
// 具体的 avdmanager 命令、参数顺序、配置文件路径等基础设施细节。
package avd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Spec 表示创建一个 AVD 所需的最小硬件 / 系统镜像参数。
type Spec struct {
	SystemImage string // SystemImage 表示 system image 包路径，如 "system-images;android-34;google_apis;x86_64"。
	Device      string // Device 表示 avdmanager 的设备型号，如 "small_phone"。
	Resolution  string // Resolution 表示模拟器屏幕分辨率，如 "1080x2400"，会写入 config.ini。
	Density     int    // Density 表示屏幕像素密度（DPI），会写入 config.ini。
}

// Manager 表示真实的 AVD 管理器，负责调用 avdmanager 命令行完成 AVD 的创建和删除。
type Manager struct {
	avdmanagerBin string // avdmanagerBin 表示 avdmanager 可执行文件绝对路径。
	avdHome       string // avdHome 表示 AVD 配置目录，对应环境变量 ANDROID_AVD_HOME。
}

// NewManager 用来按 ANDROID_HOME / ANDROID_SDK_ROOT 自动定位 avdmanager 可执行文件。
//
// 找不到 SDK 时不会立刻报错，而是返回一个仍然可构造但执行命令时会报错的 Manager，
// 这样集成测试和无 SDK 的开发机至少还能跑通基础流程。
func NewManager() *Manager {
	return &Manager{
		avdmanagerBin: locateAvdmanager(),
		avdHome:       defaultAVDHome(),
	}
}

// Create 用来调用 avdmanager 创建一个名字为 instanceName 的 AVD。
//
// 实际命令大致是：
//
//	avdmanager --silent create avd \
//	  -n <instanceName> \
//	  -k "<systemImage>" \
//	  -d <device> \
//	  --force
//
// 创建成功后，再把分辨率和 DPI 写入对应 config.ini，避免每次启动都要靠命令行覆盖。
func (m *Manager) Create(ctx context.Context, instanceName string, spec Spec) error {
	if err := m.ensureReady(); err != nil {
		return err
	}
	if strings.TrimSpace(spec.SystemImage) == "" {
		return fmt.Errorf("avd create: missing system image")
	}

	args := []string{"--silent", "create", "avd", "-n", instanceName, "-k", spec.SystemImage, "--force"}
	if spec.Device != "" {
		args = append(args, "-d", spec.Device)
	}

	cmd := exec.CommandContext(ctx, m.avdmanagerBin, args...) // #nosec G204 -- 参数来自模板配置，可信
	// avdmanager 在创建时会交互问"自定义硬件配置"，传入空 stdin 让它走默认值。
	cmd.Stdin = strings.NewReader("\n")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("avd create %s: %w (stderr=%s)", instanceName, err, stderr.String())
	}

	if err := m.applyHardwareOverrides(instanceName, spec); err != nil {
		return fmt.Errorf("avd config override %s: %w", instanceName, err)
	}

	return nil
}

// Delete 用来调用 avdmanager 删除指定名称的 AVD，对应回收清理流程。
func (m *Manager) Delete(ctx context.Context, instanceName string) error {
	if err := m.ensureReady(); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, m.avdmanagerBin, "--silent", "delete", "avd", "-n", instanceName) // #nosec G204
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("avd delete %s: %w (stderr=%s)", instanceName, err, stderr.String())
	}
	return nil
}

// ensureReady 用来在执行命令前确认 avdmanager 已被正确定位。
func (m *Manager) ensureReady() error {
	if m.avdmanagerBin == "" {
		return fmt.Errorf("avdmanager not found: set ANDROID_HOME or ANDROID_SDK_ROOT")
	}
	return nil
}

// applyHardwareOverrides 用来把分辨率和 DPI 写进 AVD 的 config.ini。
//
// avdmanager 自身不接受分辨率参数，所以这里采用"创建后改 config.ini"的常见做法：
// 在文件末尾追加 hw.lcd.* 字段，重复键 emulator 会以最后一个为准。
func (m *Manager) applyHardwareOverrides(instanceName string, spec Spec) error {
	if spec.Resolution == "" && spec.Density == 0 {
		return nil
	}
	configPath := filepath.Join(m.avdHome, instanceName+".avd", "config.ini")
	file, err := os.OpenFile(configPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	var lines []string
	if w, h, ok := splitResolution(spec.Resolution); ok {
		lines = append(lines, fmt.Sprintf("hw.lcd.width=%d", w))
		lines = append(lines, fmt.Sprintf("hw.lcd.height=%d", h))
	}
	if spec.Density > 0 {
		lines = append(lines, fmt.Sprintf("hw.lcd.density=%d", spec.Density))
	}
	if len(lines) == 0 {
		return nil
	}
	_, err = file.WriteString("\n" + strings.Join(lines, "\n") + "\n")
	return err
}

// splitResolution 用来把 "1080x2400" 这样的字符串拆成宽和高两个整数。
func splitResolution(resolution string) (int, int, bool) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(resolution)), "x")
	if len(parts) != 2 {
		return 0, 0, false
	}
	var w, h int
	if _, err := fmt.Sscanf(parts[0], "%d", &w); err != nil || w <= 0 {
		return 0, 0, false
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &h); err != nil || h <= 0 {
		return 0, 0, false
	}
	return w, h, true
}

// locateAvdmanager 用来按 ANDROID_HOME → ANDROID_SDK_ROOT → PATH 顺序找到 avdmanager。
func locateAvdmanager() string {
	candidates := []string{}
	for _, root := range []string{os.Getenv("ANDROID_HOME"), os.Getenv("ANDROID_SDK_ROOT")} {
		if root == "" {
			continue
		}
		candidates = append(candidates,
			filepath.Join(root, "cmdline-tools", "latest", "bin", "avdmanager"),
			filepath.Join(root, "cmdline-tools", "bin", "avdmanager"),
			filepath.Join(root, "tools", "bin", "avdmanager"),
		)
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	if path, err := exec.LookPath("avdmanager"); err == nil {
		return path
	}
	return ""
}

// defaultAVDHome 用来按系统约定推断 AVD 根目录，优先使用环境变量 ANDROID_AVD_HOME。
func defaultAVDHome() string {
	if home := os.Getenv("ANDROID_AVD_HOME"); home != "" {
		return home
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".android", "avd")
	}
	return ""
}
