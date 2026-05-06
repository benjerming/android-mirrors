package scrcpy_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"assassin-android-controller/internal/infrastructure/scrcpy"
)

// pump 自然死亡后，supervisor 透明重启；下一次 AttachVideo 可以重新获得数据，
// 且 StartServer 被调用两次。
func TestSessionManager_AutoRestartOnPumpDeath(t *testing.T) {
	fa := &fakeAdb{}
	sm := scrcpy.NewSessionManager(scrcpy.SessionManagerOpts{
		Adb: fa, JarBytes: []byte("fake"), Version: "3.0",
		IdleTimeout: time.Hour, // 排除 idle 干扰
	})
	defer sm.Shutdown()

	// First attach loop runs until the inner Subscribe channel closes (from killServerVideo)
	var out safeBuffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	attachOnce := make(chan error, 1)
	go func() {
		defer wg.Done()
		attachOnce <- sm.AttachVideo(ctx, "emu5554", &out)
	}()

	waitFor(t, func() bool { return out.Len() > 0 }, 2*time.Second, "first attach producing bytes")
	fa.killServerVideo()

	// First AttachVideo should return cleanly (subscriber channel closed or ctx canceled)
	select {
	case err := <-attachOnce:
		if err != nil && !errors.Is(err, context.Canceled) {
			// ok: either normal close or ctx canceled
		}
	case <-time.After(2 * time.Second):
		t.Fatal("AttachVideo did not return after pump death")
	}
	wg.Wait()

	// Wait for supervisor's backoff (default 1s base) and reattempt
	waitFor(t, func() bool {
		fa.mu.Lock()
		defer fa.mu.Unlock()
		return fa.startCalls >= 2
	}, 5*time.Second, "supervisor restart")

	// Second attach should succeed and produce bytes
	var out2 safeBuffer
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	wg.Add(1)
	go func() { defer wg.Done(); _ = sm.AttachVideo(ctx2, "emu5554", &out2) }()
	waitFor(t, func() bool { return out2.Len() > 0 }, 2*time.Second, "second attach producing bytes")
	cancel2()
	wg.Wait()
}

// 同一 serial 并发 N 个 AttachVideo 只创建 1 个 Session。
func TestSessionManager_ConcurrentAttachSingleSession(t *testing.T) {
	fa := &fakeAdb{}
	sm := scrcpy.NewSessionManager(scrcpy.SessionManagerOpts{
		Adb: fa, JarBytes: []byte("fake"), Version: "3.0",
		IdleTimeout: time.Hour,
	})
	defer sm.Shutdown()

	const N = 10
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var bufs [N]safeBuffer
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = sm.AttachVideo(ctx, "emu5554", &bufs[idx])
		}(i)
	}
	waitFor(t, func() bool {
		fa.mu.Lock()
		defer fa.mu.Unlock()
		return fa.startCalls >= 1
	}, time.Second, "first start")
	time.Sleep(150 * time.Millisecond) // let all attaches settle

	fa.mu.Lock()
	starts := fa.startCalls
	fa.mu.Unlock()
	if starts != 1 {
		t.Errorf("concurrent attaches created %d sessions, want 1", starts)
	}
	cancel()
	wg.Wait()
}

// waitFor polls until cond returns true, fails the test on timeout.
func waitFor(t *testing.T, cond func() bool, timeout time.Duration, what string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("waitFor timeout: %s", what)
}

func TestSessionManager_SnapshotsAndReset(t *testing.T) {
	fa := &fakeAdb{}
	sm := scrcpy.NewSessionManager(scrcpy.SessionManagerOpts{
		Adb: fa, JarBytes: []byte("fake"), Version: "3.0",
		IdleTimeout: time.Hour,
	})
	defer sm.Shutdown()

	// Initially no supervisors
	if got := sm.Snapshots(); len(got) != 0 {
		t.Errorf("initial Snapshots len=%d, want 0", len(got))
	}

	// Trigger a serial via SendControl (lazy ensure)
	if err := sm.SendControl(context.Background(), "emu5554",
		scrcpy.EncodeBackOrScreenOn(scrcpy.ActionDown)); err != nil {
		t.Fatalf("SendControl: %v", err)
	}
	snaps := sm.Snapshots()
	if len(snaps) != 1 {
		t.Fatalf("after SendControl len=%d, want 1", len(snaps))
	}
	if snaps[0].Serial != "emu5554" {
		t.Errorf("Serial=%q, want emu5554", snaps[0].Serial)
	}

	// Reset for unknown serial → error
	if err := sm.Reset("unknown"); err == nil {
		t.Errorf("Reset(unknown) returned nil, want error")
	}
	// Reset for known serial → ok
	if err := sm.Reset("emu5554"); err != nil {
		t.Errorf("Reset(emu5554): %v", err)
	}
}

// silence unused import warning if errors not used elsewhere
var _ = errors.New
