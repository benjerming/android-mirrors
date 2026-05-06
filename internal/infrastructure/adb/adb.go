// Package adb 封装 `adb` 命令的最小子集：APK 安装 / 卸载、locale 设置、文件 push/rm、控制事件。
//
// 所有命令都接 ctx 并设置默认超时；空 binary 时返回 ErrUnavailable，让单元测试 / 无 SDK 环境降级。
package adb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ErrUnavailable 表示 adb 二进制未配置。
var ErrUnavailable = errors.New("adb: binary path not configured")

// CommandRunner 抽象命令执行器，便于注入 fake。
type CommandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

// Client 表示 adb 命令封装。
type Client struct {
	binary  string
	timeout time.Duration
	runner  CommandRunner
}

// New 用来构造 Client。binary 为空时所有命令返回 ErrUnavailable。
func New(binary string) *Client {
	return &Client{binary: strings.TrimSpace(binary), timeout: 30 * time.Second, runner: defaultRunner}
}

// WithRunner 给单测注入命令执行器。
func (c *Client) WithRunner(r CommandRunner) *Client {
	c.runner = r
	return c
}

// WithTimeout 设置默认命令超时。
func (c *Client) WithTimeout(d time.Duration) *Client {
	if d > 0 {
		c.timeout = d
	}
	return c
}

// Install 执行 `adb -s <serial> install -r <apkPath>`。
func (c *Client) Install(ctx context.Context, serial, apkPath string) error {
	return c.run(ctx, "-s", serial, "install", "-r", apkPath)
}

// Uninstall 执行 `adb -s <serial> uninstall <pkg>`。
func (c *Client) Uninstall(ctx context.Context, serial, pkg string) error {
	return c.run(ctx, "-s", serial, "uninstall", pkg)
}

// ClearAppCache 执行 `adb -s <serial> shell pm clear <pkg>`。
func (c *Client) ClearAppCache(ctx context.Context, serial, pkg string) error {
	return c.run(ctx, "-s", serial, "shell", "pm", "clear", pkg)
}

// SetAppLocale 执行 `adb -s <serial> shell cmd locale set-app-locales <pkg> --user current --locales <lang>`。
//
// Android 13+ 才支持；旧系统会原地报 stderr，调用方按 warning 处理即可。
func (c *Client) SetAppLocale(ctx context.Context, serial, pkg, language string) error {
	return c.run(ctx, "-s", serial, "shell", "cmd", "locale", "set-app-locales",
		pkg, "--user", "current", "--locales", language)
}

// Push 执行 `adb -s <serial> push <local> <remote>`。
func (c *Client) Push(ctx context.Context, serial, localPath, remotePath string) error {
	return c.run(ctx, "-s", serial, "push", localPath, remotePath)
}

// RemoveRemote 执行 `adb -s <serial> shell rm -rf <remote>`。
func (c *Client) RemoveRemote(ctx context.Context, serial, remotePath string) error {
	return c.run(ctx, "-s", serial, "shell", "rm", "-rf", remotePath)
}

// InputTap 执行 `adb -s <serial> shell input tap <x> <y>`。
func (c *Client) InputTap(ctx context.Context, serial string, x, y int) error {
	return c.run(ctx, "-s", serial, "shell", "input", "tap", fmt.Sprintf("%d", x), fmt.Sprintf("%d", y))
}

// InputSwipe 执行 `adb -s <serial> shell input swipe`。
func (c *Client) InputSwipe(ctx context.Context, serial string, x1, y1, x2, y2, durationMs int) error {
	return c.run(ctx, "-s", serial, "shell", "input", "swipe",
		fmt.Sprintf("%d", x1), fmt.Sprintf("%d", y1),
		fmt.Sprintf("%d", x2), fmt.Sprintf("%d", y2),
		fmt.Sprintf("%d", durationMs))
}

// InputText 执行 `adb -s <serial> shell input text <text>`。
func (c *Client) InputText(ctx context.Context, serial, text string) error {
	return c.run(ctx, "-s", serial, "shell", "input", "text", text)
}

// InputKeyEvent 执行 `adb -s <serial> shell input keyevent <code>`。
func (c *Client) InputKeyEvent(ctx context.Context, serial string, code int) error {
	return c.run(ctx, "-s", serial, "shell", "input", "keyevent", fmt.Sprintf("%d", code))
}

// run 包了一层超时 + binary 校验。
func (c *Client) run(ctx context.Context, args ...string) error {
	if c.binary == "" {
		return ErrUnavailable
	}
	cctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	if _, err := c.runner(cctx, c.binary, args...); err != nil {
		return err
	}
	return nil
}

// defaultRunner 是真实命令执行器。
func defaultRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%w: %s", err, stderr.String())
	}
	return stdout.Bytes(), nil
}
