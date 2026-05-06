package scrcpy

import (
	"context"
	"errors"
	"io"
	"sync/atomic"
	"time"
)

// SessionManagerOpts 是 SessionManager 的构造参数。
type SessionManagerOpts struct {
	Adb         SessionAdb
	JarBytes    []byte
	Version     string
	Bitrate     int
	MaxFps      int
	MaxSize     int
	IdleTimeout time.Duration // 0 = 30s 默认
}

// ErrManagerClosed 表示 SessionManager 已 Shutdown。
var ErrManagerClosed = errors.New("scrcpy: session manager closed")

// SessionManager 按 serial 维护 SessionSupervisor：懒启动、空闲超时回收。
// 每个 Supervisor 内部持有一个 Session 并在 pump 死亡时自动重启。
// 内部以单 goroutine actor 实现；所有 map 操作均在 actor 内完成，无锁。
type SessionManager struct {
	opts        SessionManagerOpts
	cmds        chan any
	stop        chan struct{}
	stopped     chan struct{}
	closedFlag  atomic.Bool

	// Owned exclusively by actor goroutine — never touched from outside.
	sessions map[string]*managedSession
}

type managedSession struct {
	sup      *SessionSupervisor
	attached int
	idleGen  uint64 // bumped on each armIdle; idle ticks check gen still matches
}

// ── commands ──────────────────────────────────────────────────────────────────

type cmdAttach struct {
	serial string
	reply  chan attachReply
}

type attachReply struct {
	ms  *managedSession
	err error
}

type cmdEnsure struct { // for SendControl: get-or-create without bumping attached
	serial string
	reply  chan attachReply
}

type cmdDetach struct{ serial string }

type cmdIdleTick struct {
	serial string
	gen    uint64
}

type cmdCloseSerial struct {
	serial string
	done   chan struct{}
}

type cmdDimensions struct {
	serial string
	reply  chan dimReply
}

type dimReply struct {
	w, h uint16
	ok   bool
}

type cmdSnapshots struct {
	reply chan []SupervisorSnapshot
}

type cmdReset struct {
	serial string
	reply  chan error
}

// ── actor ─────────────────────────────────────────────────────────────────────

func (m *SessionManager) actor() {
	defer close(m.stopped)
	for {
		select {
		case <-m.stop:
			m.shutdownAll()
			return
		case raw := <-m.cmds:
			switch c := raw.(type) {
			case cmdAttach:
				ms := m.ensureSupervisor(c.serial)
				ms.attached++
				ms.idleGen++ // invalidate any pending idle ticks
				c.reply <- attachReply{ms: ms, err: nil}
			case cmdEnsure:
				ms := m.ensureSupervisor(c.serial)
				c.reply <- attachReply{ms: ms, err: nil}
				if ms.attached == 0 {
					m.armIdle(c.serial, ms)
				}
			case cmdDetach:
				if ms, ok := m.sessions[c.serial]; ok {
					if ms.attached > 0 {
						ms.attached--
					}
					if ms.attached == 0 {
						m.armIdle(c.serial, ms)
					}
				}
			case cmdIdleTick:
				if ms, ok := m.sessions[c.serial]; ok && ms.attached == 0 && ms.idleGen == c.gen {
					delete(m.sessions, c.serial)
					go ms.sup.Close()
				}
			case cmdCloseSerial:
				if ms, ok := m.sessions[c.serial]; ok {
					delete(m.sessions, c.serial)
					go ms.sup.Close()
				}
				close(c.done)
			case cmdDimensions:
				var rep dimReply
				if ms, ok := m.sessions[c.serial]; ok {
					if sess := ms.sup.CurrentSession(); sess != nil {
						rep = dimReply{w: sess.Width(), h: sess.Height(), ok: true}
					}
				}
				c.reply <- rep
			case cmdSnapshots:
				out := make([]SupervisorSnapshot, 0, len(m.sessions))
				for _, ms := range m.sessions {
					out = append(out, ms.sup.Snapshot())
				}
				c.reply <- out
			case cmdReset:
				if ms, ok := m.sessions[c.serial]; ok {
					ms.sup.Reset()
					c.reply <- nil
				} else {
					c.reply <- errors.New("scrcpy: serial not found")
				}
			}
		}
	}
}

// ensureSupervisor returns existing managedSession if present, otherwise creates one.
// Creating a supervisor is non-blocking (it spawns a goroutine; Session itself
// isn't opened until Acquire is called).
// Called only from actor goroutine.
func (m *SessionManager) ensureSupervisor(serial string) *managedSession {
	if ms, ok := m.sessions[serial]; ok {
		return ms
	}
	sup := NewSessionSupervisor(SupervisorOpts{
		Serial: serial,
		SessionOpts: SessionOpts{
			Serial:   serial,
			Adb:      m.opts.Adb,
			JarBytes: m.opts.JarBytes,
			Version:  m.opts.Version,
			Bitrate:  m.opts.Bitrate,
			MaxFps:   m.opts.MaxFps,
			MaxSize:  m.opts.MaxSize,
		},
		Policy: NewRestartPolicy(RestartPolicyOpts{}),
	})
	ms := &managedSession{sup: sup}
	m.sessions[serial] = ms
	return ms
}

func (m *SessionManager) armIdle(serial string, ms *managedSession) {
	ms.idleGen++
	gen := ms.idleGen
	time.AfterFunc(m.opts.IdleTimeout, func() {
		select {
		case m.cmds <- cmdIdleTick{serial: serial, gen: gen}:
		case <-m.stop:
		}
	})
}

func (m *SessionManager) shutdownAll() {
	all := m.sessions
	m.sessions = nil
	for _, ms := range all {
		ms.sup.Close()
	}
}

// ── public API ────────────────────────────────────────────────────────────────

// NewSessionManager 创建一个 SessionManager。
func NewSessionManager(opts SessionManagerOpts) *SessionManager {
	if opts.IdleTimeout == 0 {
		opts.IdleTimeout = 30 * time.Second
	}
	m := &SessionManager{
		opts:     opts,
		cmds:     make(chan any, 16),
		stop:     make(chan struct{}),
		stopped:  make(chan struct{}),
		sessions: make(map[string]*managedSession),
	}
	go m.actor()
	return m
}

// AttachVideo 订阅会话的 video pub/sub 流，把每个 fMP4 chunk 写到 out。
// 阻塞直到 ctx 取消或会话/pump 终止。
func (m *SessionManager) AttachVideo(ctx context.Context, serial string, out io.Writer) error {
	reply := make(chan attachReply, 1)
	select {
	case m.cmds <- cmdAttach{serial: serial, reply: reply}:
	case <-m.stop:
		return ErrManagerClosed
	case <-ctx.Done():
		return ctx.Err()
	}
	var r attachReply
	select {
	case r = <-reply:
	case <-ctx.Done():
		// Note: actor may still process the cmd and we'd lose the ms reference.
		// Acceptable: the ctx-cancel path means caller is leaving anyway.
		return ctx.Err()
	}
	if r.err != nil {
		return r.err
	}
	defer func() {
		select {
		case m.cmds <- cmdDetach{serial: serial}:
		case <-m.stop:
		}
	}()

	sess, err := r.ms.sup.Acquire(ctx)
	if err != nil {
		return err
	}
	sub, err := sess.Subscribe(ctx)
	if err != nil {
		return err
	}
	defer sub.Cancel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk, ok := <-sub.Frames:
			if !ok {
				return nil
			}
			if _, err := out.Write(chunk); err != nil {
				return err
			}
		}
	}
}

// AttachVideoRaw 与 AttachVideo 等价，但订阅 raw 路径（13B header + Annex-B）。
// 给前端 WebCodecs 用；fMP4 muxing 完全跳过。
func (m *SessionManager) AttachVideoRaw(ctx context.Context, serial string, out io.Writer) error {
	reply := make(chan attachReply, 1)
	select {
	case m.cmds <- cmdAttach{serial: serial, reply: reply}:
	case <-m.stop:
		return ErrManagerClosed
	case <-ctx.Done():
		return ctx.Err()
	}
	var r attachReply
	select {
	case r = <-reply:
	case <-ctx.Done():
		return ctx.Err()
	}
	if r.err != nil {
		return r.err
	}
	defer func() {
		select {
		case m.cmds <- cmdDetach{serial: serial}:
		case <-m.stop:
		}
	}()

	sess, err := r.ms.sup.Acquire(ctx)
	if err != nil {
		return err
	}
	sub, err := sess.SubscribeRaw(ctx)
	if err != nil {
		return err
	}
	defer sub.Cancel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk, ok := <-sub.Frames:
			if !ok {
				return nil
			}
			if _, err := out.Write(chunk); err != nil {
				return err
			}
		}
	}
}

// SendControl 把控制字节写到 control socket；任何活动都会重置 idle 计时。
func (m *SessionManager) SendControl(ctx context.Context, serial string, payload []byte) error {
	reply := make(chan attachReply, 1)
	select {
	case m.cmds <- cmdEnsure{serial: serial, reply: reply}:
	case <-m.stop:
		return ErrManagerClosed
	case <-ctx.Done():
		return ctx.Err()
	}
	var r attachReply
	select {
	case r = <-reply:
	case <-ctx.Done():
		return ctx.Err()
	}
	if r.err != nil {
		return r.err
	}
	sess, err := r.ms.sup.Acquire(ctx)
	if err != nil {
		return err
	}
	return sess.SendControl(payload)
}

// Dimensions 返回设备屏幕分辨率；ok=false 表示尚未建立会话或 supervisor 未就绪。
func (m *SessionManager) Dimensions(serial string) (uint16, uint16, bool) {
	reply := make(chan dimReply, 1)
	select {
	case m.cmds <- cmdDimensions{serial: serial, reply: reply}:
	case <-m.stop:
		return 0, 0, false
	}
	r := <-reply
	return r.w, r.h, r.ok
}

// Close 主动关闭某个 serial 的 session。幂等。
func (m *SessionManager) Close(serial string) {
	done := make(chan struct{})
	select {
	case m.cmds <- cmdCloseSerial{serial: serial, done: done}:
	case <-m.stop:
		return
	}
	<-done
}

// Snapshots returns runtime status of every supervisor currently tracked.
func (m *SessionManager) Snapshots() []SupervisorSnapshot {
	reply := make(chan []SupervisorSnapshot, 1)
	select {
	case m.cmds <- cmdSnapshots{reply: reply}:
	case <-m.stop:
		return nil
	}
	select {
	case s := <-reply:
		return s
	case <-m.stop:
		return nil
	}
}

// Reset clears the RestartPolicy on the supervisor for the given serial.
// Returns an error if no supervisor exists for that serial.
func (m *SessionManager) Reset(serial string) error {
	reply := make(chan error, 1)
	select {
	case m.cmds <- cmdReset{serial: serial, reply: reply}:
	case <-m.stop:
		return ErrManagerClosed
	}
	select {
	case err := <-reply:
		return err
	case <-m.stop:
		return ErrManagerClosed
	}
}

// Shutdown 关闭所有 session。SessionManager 之后不可再用。
func (m *SessionManager) Shutdown() {
	if !m.closedFlag.CompareAndSwap(false, true) {
		<-m.stopped
		return
	}
	close(m.stop)
	<-m.stopped
}
