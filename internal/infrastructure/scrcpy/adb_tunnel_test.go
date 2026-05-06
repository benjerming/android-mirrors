package scrcpy_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"assassin-android-controller/internal/infrastructure/scrcpy"
)

func TestAdbTunnel_Push_ComposesArgs(t *testing.T) {
	var got []string
	tun := scrcpy.NewAdbTunnel("/fake/adb").WithRunner(func(_ context.Context, _ string, args ...string) ([]byte, error) {
		got = args
		return nil, nil
	})
	if err := tun.Push(context.Background(), "emu5554", "/local/scrcpy.jar", "/data/local/tmp/scrcpy-server.jar"); err != nil {
		t.Fatalf("Push: %v", err)
	}
	joined := strings.Join(got, " ")
	for _, want := range []string{"-s emu5554", "push", "/local/scrcpy.jar", "/data/local/tmp/scrcpy-server.jar"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in %q", want, joined)
		}
	}
}

func TestAdbTunnel_Reverse_ComposesArgs(t *testing.T) {
	var got []string
	tun := scrcpy.NewAdbTunnel("/fake/adb").WithRunner(func(_ context.Context, _ string, args ...string) ([]byte, error) {
		got = args
		return nil, nil
	})
	if err := tun.Reverse(context.Background(), "emu5554", "scrcpy", 41234); err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if !strings.Contains(strings.Join(got, " "), "reverse localabstract:scrcpy tcp:41234") {
		t.Errorf("got %v", got)
	}
}

func TestAdbTunnel_RemoveReverse(t *testing.T) {
	var got []string
	tun := scrcpy.NewAdbTunnel("/fake/adb").WithRunner(func(_ context.Context, _ string, args ...string) ([]byte, error) {
		got = args
		return nil, nil
	})
	_ = tun.RemoveReverse(context.Background(), "emu5554", "scrcpy")
	if !strings.Contains(strings.Join(got, " "), "reverse --remove localabstract:scrcpy") {
		t.Errorf("got %v", got)
	}
}

func TestAdbTunnel_StartServer_ArgsContainCmdAndKeyParams(t *testing.T) {
	var got []string
	tun := scrcpy.NewAdbTunnel("/fake/adb").WithRunner(func(_ context.Context, _ string, args ...string) ([]byte, error) {
		got = args
		return nil, nil
	})
	cmd, err := tun.StartServer(context.Background(), "emu5554", scrcpy.ServerStartOpts{
		Version:    "3.0",
		BitrateBps: 4_000_000,
		MaxFps:     30,
	})
	if err != nil {
		t.Fatalf("StartServer: %v", err)
	}
	if cmd == nil {
		t.Fatalf("expected non-nil cmd")
	}
	joined := strings.Join(got, " ")
	for _, want := range []string{
		"shell", "CLASSPATH=/data/local/tmp/scrcpy-server.jar",
		"app_process", "/", "com.genymobile.scrcpy.Server", "3.0",
		"tunnel_forward=false", "audio=false", "control=true",
		"video_bit_rate=4000000", "max_fps=30",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in %q", want, joined)
		}
	}
}

func TestAdbTunnel_Unavailable(t *testing.T) {
	tun := scrcpy.NewAdbTunnel("")
	if err := tun.Push(context.Background(), "emu1", "a", "b"); !errors.Is(err, scrcpy.ErrAdbUnavailable) {
		t.Errorf("expect ErrAdbUnavailable, got %v", err)
	}
	if _, err := tun.StartServer(context.Background(), "emu1", scrcpy.ServerStartOpts{Version: "3.0"}); !errors.Is(err, scrcpy.ErrAdbUnavailable) {
		t.Errorf("expect ErrAdbUnavailable on StartServer, got %v", err)
	}
}
