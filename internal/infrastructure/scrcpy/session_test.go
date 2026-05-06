package scrcpy_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"os/exec"
	"strconv"
	"sync"
	"testing"
	"time"

	"assassin-android-controller/internal/infrastructure/scrcpy"
)

// fakeAdb 实现 SessionAdb；StartServer 不真起进程，
// 而是模拟 scrcpy-server 行为：先 connect video，再 connect control，
// 然后发 device meta + codec meta + 一帧 SPS+PPS+IDR。
type fakeAdb struct {
	mu             sync.Mutex
	pushed         bool
	reverseSet     bool
	tcpPort        int
	startCalls     int
	startErr       error // if non-nil, StartServer returns this
	controlReader  func(net.Conn)
	suppressFrames bool
	serverVideo    net.Conn
}

func (f *fakeAdb) killServerVideo() {
	f.mu.Lock()
	c := f.serverVideo
	f.mu.Unlock()
	if c != nil {
		_ = c.Close()
	}
}

func (f *fakeAdb) Push(_ context.Context, _, _, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pushed = true
	return nil
}

func (f *fakeAdb) Reverse(_ context.Context, _, _ string, port int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reverseSet = true
	f.tcpPort = port
	return nil
}

func (f *fakeAdb) RemoveReverse(_ context.Context, _, _ string) error { return nil }

func (f *fakeAdb) StartServer(_ context.Context, _ string, _ scrcpy.ServerStartOpts) (*exec.Cmd, error) {
	f.mu.Lock()
	f.startCalls++
	if f.startErr != nil {
		err := f.startErr
		f.mu.Unlock()
		return nil, err
	}
	port := f.tcpPort
	cr := f.controlReader
	noFrames := f.suppressFrames
	f.mu.Unlock()
	go simulateServer(f, port, cr, noFrames)
	return exec.Command("true"), nil
}

func simulateServer(f *fakeAdb, port int, controlReader func(net.Conn), suppressFrames bool) {
	time.Sleep(20 * time.Millisecond)
	video, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		return
	}
	f.mu.Lock()
	f.serverVideo = video
	f.mu.Unlock()
	control, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		video.Close()
		return
	}
	// scrcpy 3.0 video preamble: 64 字节设备名（NUL 填充，无 dummy byte），
	// 与 readMeta 实测确认的协议一致。
	name := make([]byte, 64)
	copy(name, "sdk_gphone64_x86_64")
	video.Write(name)
	// codec meta: codecId(4) + width(4) + height(4) = 12B
	cm := make([]byte, 12)
	binary.BigEndian.PutUint32(cm[0:4], 0x68323634) // 'h264'
	binary.BigEndian.PutUint32(cm[4:8], 1280)
	binary.BigEndian.PutUint32(cm[8:12], 720)
	video.Write(cm)
	if !suppressFrames {
		// 1 frame: header (pts 8B + size 4B) + SPS+PPS+IDR
		frame := bytes.Join([][]byte{testSPS, testPPS, testIDR}, nil)
		hdr := make([]byte, 12)
		binary.BigEndian.PutUint64(hdr[0:8], 0)
		binary.BigEndian.PutUint32(hdr[8:12], uint32(len(frame)))
		video.Write(hdr)
		video.Write(frame)
	}
	if controlReader != nil {
		controlReader(control)
	} else {
		go io.Copy(io.Discard, control)
	}
}

func TestSession_StartAndAttachVideo(t *testing.T) {
	fa := &fakeAdb{}
	sess, err := scrcpy.OpenSession(context.Background(), scrcpy.SessionOpts{
		Serial:   "emu5554",
		Adb:      fa,
		JarBytes: []byte("fake-jar-bytes"),
		Version:  "3.0",
	})
	if err != nil {
		t.Fatalf("OpenSession: %v", err)
	}
	defer sess.Close()

	fa.mu.Lock()
	pushed, reverseSet, starts := fa.pushed, fa.reverseSet, fa.startCalls
	fa.mu.Unlock()
	if !pushed || !reverseSet || starts != 1 {
		t.Errorf("adb steps not invoked: pushed=%v reverse=%v starts=%d", pushed, reverseSet, starts)
	}
	if sess.Width() != 1280 || sess.Height() != 720 {
		t.Errorf("dimensions wrong: %dx%d", sess.Width(), sess.Height())
	}

	var out safeBuffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sub, err := sess.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer sub.Cancel()
	go func() {
		for chunk := range sub.Frames {
			out.Write(chunk)
		}
	}()

	deadline := time.After(2 * time.Second)
	for out.Len() < 200 {
		select {
		case <-deadline:
			t.Fatalf("video not pumped within 2s, got %d bytes", out.Len())
		case <-time.After(20 * time.Millisecond):
		}
	}
	if !bytes.Contains(out.Bytes(), []byte("ftyp")) {
		t.Errorf("output missing ftyp")
	}
	if !bytes.Contains(out.Bytes(), []byte("moof")) {
		t.Errorf("output missing moof (no fragment emitted)")
	}
}

func TestSession_SendControlWritesBytes(t *testing.T) {
	gotCtrl := make(chan []byte, 4)
	fa := &fakeAdb{
		suppressFrames: true,
		controlReader: func(c net.Conn) {
			buf := make([]byte, 256)
			for {
				c.SetReadDeadline(time.Now().Add(2 * time.Second))
				n, err := c.Read(buf)
				if n > 0 {
					b := make([]byte, n)
					copy(b, buf[:n])
					gotCtrl <- b
				}
				if err != nil {
					return
				}
			}
		},
	}
	sess, err := scrcpy.OpenSession(context.Background(), scrcpy.SessionOpts{
		Serial: "emu5554", Adb: fa, JarBytes: []byte("fake"), Version: "3.0",
	})
	if err != nil {
		t.Fatalf("OpenSession: %v", err)
	}
	defer sess.Close()

	if err := sess.SendControl(scrcpy.EncodeBackOrScreenOn(scrcpy.ActionDown)); err != nil {
		t.Fatalf("SendControl: %v", err)
	}
	select {
	case got := <-gotCtrl:
		if !bytes.Equal(got, []byte{0x04, 0x00}) {
			t.Errorf("got %x, want 0400", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("control bytes not received within 2s")
	}
}

// TestSession_ConcurrentSubscribers 锁死 pub/sub fan-out 的核心契约：
// 两个并发订阅者都能拿到 init + IDR fragment；后来者不会让前一个失效。
// 这是修掉 "single-subscriber + 踢人 + 共享 socket SetReadDeadline 污染" bug 的回归点。
func TestSession_ConcurrentSubscribers(t *testing.T) {
	fa := &fakeAdb{}
	sess, err := scrcpy.OpenSession(context.Background(), scrcpy.SessionOpts{
		Serial: "emu5554", Adb: fa, JarBytes: []byte("fake"), Version: "3.0",
	})
	if err != nil {
		t.Fatalf("OpenSession: %v", err)
	}
	defer sess.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	subA, err := sess.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe A: %v", err)
	}
	defer subA.Cancel()

	// 订阅者 A 先吃完 init + key，确认 pump 已经把帧广播过一轮。
	var outA safeBuffer
	collect := func(out *safeBuffer, sub *scrcpy.VideoSubscription) {
		deadline := time.After(2 * time.Second)
		for !bytes.Contains(out.Bytes(), []byte("ftyp")) ||
			!bytes.Contains(out.Bytes(), []byte("moof")) {
			select {
			case <-deadline:
				t.Errorf("subscriber stalled at %d bytes; got %q", out.Len(), out.Bytes())
				return
			case chunk, ok := <-sub.Frames:
				if !ok {
					t.Errorf("subscriber channel closed at %d bytes", out.Len())
					return
				}
				out.Write(chunk)
			}
		}
	}
	collect(&outA, subA)

	// 第二个订阅者：必须立刻拿到缓存的 init + key fragment，
	// 不依赖 scrcpy 再发任何新帧。这正是旧实现做不到、且会黑屏的场景。
	subB, err := sess.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe B: %v", err)
	}
	defer subB.Cancel()
	var outB safeBuffer
	collect(&outB, subB)

	// A 的 channel 不能因为 B 的接入被关掉。
	select {
	case _, ok := <-subA.Frames:
		if !ok {
			t.Errorf("A.Frames was closed after B subscribed")
		}
	case <-time.After(50 * time.Millisecond):
		// 没新数据正常；只要没收到 close 信号就行
	}
}

// safeBuffer 是 goroutine-safe 的 bytes.Buffer。
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *safeBuffer) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Len()
}

func (s *safeBuffer) Bytes() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]byte, s.buf.Len())
	copy(out, s.buf.Bytes())
	return out
}

func TestSession_StateLifecycle(t *testing.T) {
	fa := &fakeAdb{}
	s, err := scrcpy.OpenSession(context.Background(), scrcpy.SessionOpts{
		Serial: "emu5554", Adb: fa, JarBytes: []byte("fake"), Version: "3.0",
	})
	if err != nil {
		t.Fatalf("OpenSession: %v", err)
	}
	if got := s.State(); got != scrcpy.StateRunning {
		t.Errorf("after Open: state=%s, want running", got)
	}
	select {
	case <-s.Done():
		t.Fatal("Done() closed before Close()")
	default:
	}
	s.Close()
	select {
	case <-s.Done():
	case <-time.After(time.Second):
		t.Fatal("Done() not closed within 1s after Close()")
	}
	if got := s.State(); got != scrcpy.StateDead {
		t.Errorf("after Close: state=%s, want dead", got)
	}
}

func TestSession_PumpExitTransitionsToDead(t *testing.T) {
	fa := &fakeAdb{}
	s, err := scrcpy.OpenSession(context.Background(), scrcpy.SessionOpts{
		Serial: "emu5554", Adb: fa, JarBytes: []byte("fake"), Version: "3.0",
	})
	if err != nil {
		t.Fatalf("OpenSession: %v", err)
	}
	defer s.Close()

	// Wait for pump to actually have read the meta and started reading frames
	// — otherwise killServerVideo races with simulateServer not yet having
	// registered the conn. Subscribe + first chunk is a reliable barrier.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sub, err := s.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	select {
	case <-sub.Frames:
	case <-time.After(time.Second):
		t.Fatal("no frame within 1s")
	}
	sub.Cancel()

	fa.killServerVideo()

	select {
	case <-s.Done():
	case <-time.After(time.Second):
		t.Fatal("pump exit did not signal Done()")
	}
	if got := s.State(); got != scrcpy.StateDead {
		t.Errorf("state=%s, want dead", got)
	}
}
