// Package aapt2 提供"用 aapt2 二进制解析 APK 包名"的最小封装。
//
// 设计目标只覆盖 spec §3.3.4 的需要：上传 APK 时拿到 packageName 落库。
// 其他 aapt2 能力（resource 列表、签名、版本等）按需再补。
package aapt2

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Parser 表示 aapt2 命令封装。binary 为空时退化为"返回 ErrUnavailable"，
// 让上层在 CI / 本地未装 aapt2 的环境也能优雅降级。
type Parser struct {
	binary string
	runner func(ctx context.Context, name string, args ...string) ([]byte, error)
}

// ErrUnavailable 表示 aapt2 二进制未配置；调用方可以选择跳过包名提取。
var ErrUnavailable = errors.New("aapt2: binary path not configured")

// New 用来构造 Parser；binaryPath 为空时所有方法返回 ErrUnavailable。
func New(binaryPath string) *Parser {
	return &Parser{binary: strings.TrimSpace(binaryPath), runner: defaultRunner}
}

// WithRunner 给单测注入命令执行器，避免依赖真实 aapt2。
func (p *Parser) WithRunner(r func(ctx context.Context, name string, args ...string) ([]byte, error)) *Parser {
	p.runner = r
	return p
}

// PackageName 解析 APK 包名。底层执行 `aapt2 dump packagename <apk>`。
func (p *Parser) PackageName(ctx context.Context, apkPath string) (string, error) {
	if p.binary == "" {
		return "", ErrUnavailable
	}
	out, err := p.runner(ctx, p.binary, "dump", "packagename", apkPath)
	if err != nil {
		return "", fmt.Errorf("aapt2 dump packagename: %w", err)
	}
	pkg := strings.TrimSpace(string(out))
	if pkg == "" {
		return "", errors.New("aapt2: empty package name")
	}
	// aapt2 dump packagename 直接输出包名行；保险起见取第一行。
	if idx := strings.IndexByte(pkg, '\n'); idx >= 0 {
		pkg = pkg[:idx]
	}
	return strings.TrimSpace(pkg), nil
}

// defaultRunner 是真实命令执行器，单测会用 WithRunner 替换。
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
