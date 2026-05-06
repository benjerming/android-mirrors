package aapt2_test

import (
	"context"
	"errors"
	"testing"

	"assassin-android-controller/internal/infrastructure/aapt2"
)

func TestPackageName_ParsesSingleLine(t *testing.T) {
	p := aapt2.New("/fake/aapt2").WithRunner(func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return []byte("com.example.app\n"), nil
	})
	pkg, err := p.PackageName(context.Background(), "/tmp/foo.apk")
	if err != nil {
		t.Fatalf("PackageName: %v", err)
	}
	if pkg != "com.example.app" {
		t.Errorf("expect com.example.app, got %q", pkg)
	}
}

func TestPackageName_TrimsTrailingOutput(t *testing.T) {
	p := aapt2.New("/fake/aapt2").WithRunner(func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return []byte("com.foo.bar\n  some other line\n"), nil
	})
	pkg, _ := p.PackageName(context.Background(), "x.apk")
	if pkg != "com.foo.bar" {
		t.Errorf("expect com.foo.bar, got %q", pkg)
	}
}

func TestPackageName_UnavailableWhenBinaryEmpty(t *testing.T) {
	p := aapt2.New("")
	_, err := p.PackageName(context.Background(), "x.apk")
	if !errors.Is(err, aapt2.ErrUnavailable) {
		t.Errorf("expect ErrUnavailable, got %v", err)
	}
}
