package scrcpy_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"assassin-android-controller/internal/infrastructure/scrcpy"
)

func TestSessionManager_LazyStart(t *testing.T) {
	fa := &fakeAdb{}
	sm := scrcpy.NewSessionManager(scrcpy.SessionManagerOpts{
		Adb: fa, JarBytes: []byte("fake"), Version: "3.0",
		IdleTimeout: 200 * time.Millisecond,
	})
	defer sm.Shutdown()

	fa.mu.Lock()
	startsBefore := fa.startCalls
	fa.mu.Unlock()
	if startsBefore != 0 {
		t.Errorf("expected 0 starts before any call, got %d", startsBefore)
	}

	if err := sm.SendControl(context.Background(), "emu5554",
		scrcpy.EncodeBackOrScreenOn(scrcpy.ActionDown)); err != nil {
		t.Fatalf("SendControl: %v", err)
	}
	fa.mu.Lock()
	starts := fa.startCalls
	fa.mu.Unlock()
	if starts != 1 {
		t.Errorf("expected 1 start after SendControl, got %d", starts)
	}
}

func TestSessionManager_IdleTimeoutClosesSession(t *testing.T) {
	fa := &fakeAdb{}
	sm := scrcpy.NewSessionManager(scrcpy.SessionManagerOpts{
		Adb: fa, JarBytes: []byte("fake"), Version: "3.0",
		IdleTimeout: 80 * time.Millisecond,
	})
	defer sm.Shutdown()

	if err := sm.SendControl(context.Background(), "emu5554",
		scrcpy.EncodeCollapsePanels()); err != nil {
		t.Fatalf("SendControl 1: %v", err)
	}
	time.Sleep(250 * time.Millisecond)
	if err := sm.SendControl(context.Background(), "emu5554",
		scrcpy.EncodeCollapsePanels()); err != nil {
		t.Fatalf("SendControl 2: %v", err)
	}

	fa.mu.Lock()
	starts := fa.startCalls
	fa.mu.Unlock()
	if starts < 2 {
		t.Errorf("expected reconnect after idle, got %d starts", starts)
	}
}

func TestSessionManager_AttachKeepsSessionAlive(t *testing.T) {
	fa := &fakeAdb{}
	sm := scrcpy.NewSessionManager(scrcpy.SessionManagerOpts{
		Adb: fa, JarBytes: []byte("fake"), Version: "3.0",
		IdleTimeout: 60 * time.Millisecond,
	})
	defer sm.Shutdown()

	var out safeBuffer
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = sm.AttachVideo(ctx, "emu5554", &out)
	}()
	// 等 attach 起来
	for i := 0; i < 50 && out.Len() == 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	// 持续到 idle timeout 之外
	time.Sleep(150 * time.Millisecond)
	// 再来一次 SendControl，不应触发新的 start（session 仍因 attach 活着）
	_ = sm.SendControl(context.Background(), "emu5554", scrcpy.EncodeCollapsePanels())
	fa.mu.Lock()
	starts := fa.startCalls
	fa.mu.Unlock()
	if starts != 1 {
		t.Errorf("expected exactly 1 start while attached, got %d", starts)
	}
	cancel()
	wg.Wait()
}
