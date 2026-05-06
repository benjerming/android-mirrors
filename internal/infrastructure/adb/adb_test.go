package adb_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"assassin-android-controller/internal/infrastructure/adb"
)

func TestClient_UnavailableWhenBinaryEmpty(t *testing.T) {
	c := adb.New("")
	if err := c.Install(context.Background(), "emu1", "/x.apk"); !errors.Is(err, adb.ErrUnavailable) {
		t.Errorf("expect ErrUnavailable, got %v", err)
	}
}

func TestClient_SetAppLocale_ComposesCommand(t *testing.T) {
	var capturedArgs []string
	c := adb.New("/fake/adb").WithRunner(func(_ context.Context, _ string, args ...string) ([]byte, error) {
		capturedArgs = args
		return nil, nil
	})
	if err := c.SetAppLocale(context.Background(), "emu5554", "com.foo", "zh-CN"); err != nil {
		t.Fatalf("SetAppLocale: %v", err)
	}
	joined := strings.Join(capturedArgs, " ")
	for _, want := range []string{"-s", "emu5554", "shell", "cmd", "locale", "set-app-locales", "com.foo", "zh-CN"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expect %q in %q", want, joined)
		}
	}
}

func TestClient_InputTap_PassesCoords(t *testing.T) {
	var capturedArgs []string
	c := adb.New("/fake/adb").WithRunner(func(_ context.Context, _ string, args ...string) ([]byte, error) {
		capturedArgs = args
		return nil, nil
	})
	_ = c.InputTap(context.Background(), "emu5554", 100, 200)
	if !strings.Contains(strings.Join(capturedArgs, " "), "tap 100 200") {
		t.Errorf("expect tap 100 200, got %v", capturedArgs)
	}
}
