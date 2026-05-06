package scrcpy_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"assassin-android-controller/internal/infrastructure/scrcpy"
)

// supervisor 在 session 死亡后自动重启
func TestSupervisor_AutoRestartAfterPumpDeath(t *testing.T) {
	fa := &fakeAdb{}
	sup := scrcpy.NewSessionSupervisor(scrcpy.SupervisorOpts{
		Serial: "emu5554",
		SessionOpts: scrcpy.SessionOpts{
			Serial: "emu5554", Adb: fa, JarBytes: []byte("fake"), Version: "3.0",
		},
		Policy: scrcpy.NewRestartPolicy(scrcpy.RestartPolicyOpts{
			BaseDelay: 30 * time.Millisecond, MaxDelay: 60 * time.Millisecond, MaxAttempts: 5,
		}),
	})
	defer sup.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	sess1, err := sup.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire 1: %v", err)
	}
	if sess1 == nil {
		t.Fatal("Acquire 1 returned nil session")
	}

	fa.killServerVideo()
	select {
	case <-sess1.Done():
	case <-time.After(time.Second):
		t.Fatal("first session did not die within 1s after killServerVideo")
	}

	// Wait for supervisor to restart and produce a new session
	waitFor(t, func() bool {
		fa.mu.Lock(); defer fa.mu.Unlock()
		return fa.startCalls >= 2
	}, 2*time.Second, "supervisor restart")

	sess2, err := sup.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire 2: %v", err)
	}
	if sess2 == nil || sess2 == sess1 {
		t.Errorf("expected new session after restart; sess2=%p sess1=%p", sess2, sess1)
	}
}

// 连续失败超过 MaxAttempts → Failed 状态，Acquire 返回 ErrSessionFailed
func TestSupervisor_CircuitOpensAfterMaxFailures(t *testing.T) {
	fa := &fakeAdb{startErr: errors.New("simulated start failure")}
	sup := scrcpy.NewSessionSupervisor(scrcpy.SupervisorOpts{
		Serial: "emu5554",
		SessionOpts: scrcpy.SessionOpts{
			Serial: "emu5554", Adb: fa, JarBytes: []byte("fake"), Version: "3.0",
		},
		Policy: scrcpy.NewRestartPolicy(scrcpy.RestartPolicyOpts{
			BaseDelay: 5 * time.Millisecond, MaxDelay: 10 * time.Millisecond, MaxAttempts: 3,
		}),
	})
	defer sup.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := sup.Acquire(ctx)
	if !errors.Is(err, scrcpy.ErrSessionFailed) {
		t.Errorf("expected ErrSessionFailed, got %v", err)
	}
}

// Reset 后 supervisor 从 Failed 恢复
func TestSupervisor_ResetRecoversFromFailed(t *testing.T) {
	callCount := 0
	fa := &fakeAdb{}
	sup := scrcpy.NewSessionSupervisor(scrcpy.SupervisorOpts{
		Serial: "emu5554",
		SessionOpts: scrcpy.SessionOpts{
			Serial: "emu5554", Adb: fa, JarBytes: []byte("fake"), Version: "3.0",
		},
		Policy: scrcpy.NewRestartPolicy(scrcpy.RestartPolicyOpts{
			BaseDelay: 5 * time.Millisecond, MaxDelay: 10 * time.Millisecond, MaxAttempts: 2,
		}),
		OpenFunc: func(ctx context.Context, opts scrcpy.SessionOpts) (*scrcpy.Session, error) {
			callCount++
			if callCount <= 3 {
				return nil, errors.New("simulated")
			}
			return scrcpy.OpenSession(ctx, opts)
		},
	})
	defer sup.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := sup.Acquire(ctx)
	if !errors.Is(err, scrcpy.ErrSessionFailed) {
		t.Fatalf("expected ErrSessionFailed pre-reset, got %v", err)
	}

	sup.Reset()

	sess, err := sup.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire after Reset: %v", err)
	}
	if sess == nil {
		t.Fatal("expected non-nil session after reset")
	}
}

func TestSupervisor_SnapshotIdle(t *testing.T) {
	fa := &fakeAdb{}
	sup := scrcpy.NewSessionSupervisor(scrcpy.SupervisorOpts{
		Serial: "emu5554",
		SessionOpts: scrcpy.SessionOpts{
			Serial: "emu5554", Adb: fa, JarBytes: []byte("fake"), Version: "3.0",
		},
	})
	defer sup.Close()
	snap := sup.Snapshot()
	if snap.Serial != "emu5554" {
		t.Errorf("Serial=%q, want emu5554", snap.Serial)
	}
	if snap.State != "idle" {
		t.Errorf("State=%q, want idle", snap.State)
	}
	if snap.SessionAlive {
		t.Errorf("SessionAlive=true, want false")
	}
}

func TestSupervisor_SnapshotRunning(t *testing.T) {
	fa := &fakeAdb{}
	sup := scrcpy.NewSessionSupervisor(scrcpy.SupervisorOpts{
		Serial: "emu5554",
		SessionOpts: scrcpy.SessionOpts{
			Serial: "emu5554", Adb: fa, JarBytes: []byte("fake"), Version: "3.0",
		},
	})
	defer sup.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := sup.Acquire(ctx); err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	snap := sup.Snapshot()
	if snap.State != "running" {
		t.Errorf("State=%q, want running", snap.State)
	}
	if !snap.SessionAlive {
		t.Errorf("SessionAlive=false")
	}
	if snap.Width != 1280 || snap.Height != 720 {
		t.Errorf("dimensions %dx%d, want 1280x720", snap.Width, snap.Height)
	}
	if snap.StartedAt.IsZero() {
		t.Errorf("StartedAt is zero")
	}
}

func TestSupervisor_SnapshotFailedHasLastError(t *testing.T) {
	fa := &fakeAdb{startErr: errors.New("boom")}
	sup := scrcpy.NewSessionSupervisor(scrcpy.SupervisorOpts{
		Serial: "emu5554",
		SessionOpts: scrcpy.SessionOpts{
			Serial: "emu5554", Adb: fa, JarBytes: []byte("fake"), Version: "3.0",
		},
		Policy: scrcpy.NewRestartPolicy(scrcpy.RestartPolicyOpts{
			BaseDelay: 5 * time.Millisecond, MaxDelay: 10 * time.Millisecond, MaxAttempts: 2,
		}),
	})
	defer sup.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, _ = sup.Acquire(ctx) // expected to fail
	waitFor(t, func() bool { return sup.State() == scrcpy.SupFailed }, time.Second, "supervisor failed")
	snap := sup.Snapshot()
	if snap.State != "failed" {
		t.Errorf("State=%q, want failed", snap.State)
	}
	if snap.LastError == "" {
		t.Errorf("LastError empty; want last failure message")
	}
	if snap.Attempts < 1 {
		t.Errorf("Attempts=%d, want >=1", snap.Attempts)
	}
}

// State() 在生命周期中正确转换
func TestSupervisor_StateTransitions(t *testing.T) {
	fa := &fakeAdb{}
	sup := scrcpy.NewSessionSupervisor(scrcpy.SupervisorOpts{
		Serial: "emu5554",
		SessionOpts: scrcpy.SessionOpts{
			Serial: "emu5554", Adb: fa, JarBytes: []byte("fake"), Version: "3.0",
		},
	})
	defer sup.Close()
	if got := sup.State(); got != scrcpy.SupIdle {
		t.Errorf("initial state=%s, want idle", got)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := sup.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if got := sup.State(); got != scrcpy.SupRunning {
		t.Errorf("after Acquire: state=%s, want running", got)
	}
}
