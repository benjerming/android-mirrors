package scrcpy

import (
	"context"
	"errors"
	"sync/atomic"
	"time"
)

var ErrSessionFailed = errors.New("scrcpy: session permanently failed")

// SupervisorSnapshot is a read-only runtime view of a SessionSupervisor.
// Used by the debug endpoint and operators to inspect "why is X stuck".
type SupervisorSnapshot struct {
	Serial       string    `json:"serial"`
	State        string    `json:"state"`
	Attempts     int       `json:"attempts"`
	LastError    string    `json:"last_error,omitempty"`
	SessionAlive bool      `json:"session_alive"`
	Width        uint16    `json:"width,omitempty"`
	Height       uint16    `json:"height,omitempty"`
	StartedAt    time.Time `json:"started_at,omitempty"`
}

type SupervisorState int32

const (
	SupIdle SupervisorState = iota
	SupStarting
	SupRunning
	SupBackoff
	SupFailed
	SupClosed
)

func (s SupervisorState) String() string {
	return [...]string{"idle", "starting", "running", "backoff", "failed", "closed"}[s]
}

type SupervisorOpts struct {
	Serial      string
	SessionOpts SessionOpts
	Policy      *RestartPolicy
	// OpenFunc is the OpenSession entry point; when nil uses real OpenSession.
	// Tests inject a fake to drive failure paths deterministically.
	OpenFunc func(ctx context.Context, opts SessionOpts) (*Session, error)
}

type SessionSupervisor struct {
	opts    SupervisorOpts
	state   atomic.Int32

	cmds    chan any
	stop    chan struct{}
	stopped chan struct{}
	closed  atomic.Bool
}

type supAcquireReq struct {
	ctx   context.Context
	reply chan supAcquireReply
}
type supAcquireReply struct {
	sess *Session
	err  error
}
type supResetReq struct {
	done chan struct{}
}

type supCurrentReq struct {
	reply chan *Session
}

type supSnapshotReq struct {
	reply chan SupervisorSnapshot
}

func NewSessionSupervisor(opts SupervisorOpts) *SessionSupervisor {
	if opts.Policy == nil {
		opts.Policy = NewRestartPolicy(RestartPolicyOpts{})
	}
	if opts.OpenFunc == nil {
		opts.OpenFunc = OpenSession
	}
	s := &SessionSupervisor{
		opts:    opts,
		cmds:    make(chan any, 8),
		stop:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
	s.state.Store(int32(SupIdle))
	go s.run()
	return s
}

func (s *SessionSupervisor) State() SupervisorState {
	return SupervisorState(s.state.Load())
}

// Acquire returns the currently-alive Session for this serial, blocking through
// any in-flight backoff/start. Returns ErrSessionFailed if the circuit is open.
func (s *SessionSupervisor) Acquire(ctx context.Context) (*Session, error) {
	reply := make(chan supAcquireReply, 1)
	select {
	case s.cmds <- supAcquireReq{ctx: ctx, reply: reply}:
	case <-s.stop:
		return nil, ErrManagerClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	select {
	case r := <-reply:
		return r.sess, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.stop:
		return nil, ErrManagerClosed
	}
}

// CurrentSession returns the running Session if the supervisor is in the
// Running state, or nil otherwise. Safe to call from any goroutine.
func (s *SessionSupervisor) CurrentSession() *Session {
	reply := make(chan *Session, 1)
	select {
	case s.cmds <- supCurrentReq{reply: reply}:
	case <-s.stop:
		return nil
	}
	select {
	case sess := <-reply:
		return sess
	case <-s.stop:
		return nil
	}
}

// Snapshot returns a point-in-time read-only view of this supervisor's state.
// Safe to call from any goroutine.
func (s *SessionSupervisor) Snapshot() SupervisorSnapshot {
	reply := make(chan SupervisorSnapshot, 1)
	select {
	case s.cmds <- supSnapshotReq{reply: reply}:
	case <-s.stop:
		return SupervisorSnapshot{Serial: s.opts.Serial, State: "closed"}
	}
	select {
	case snap := <-reply:
		return snap
	case <-s.stop:
		return SupervisorSnapshot{Serial: s.opts.Serial, State: "closed"}
	}
}

// Reset clears the failure counter and lets Acquire restart from scratch.
// Used as the manual recovery path when policy has opened the circuit.
func (s *SessionSupervisor) Reset() {
	done := make(chan struct{})
	select {
	case s.cmds <- supResetReq{done: done}:
	case <-s.stop:
		return
	}
	<-done
}

func (s *SessionSupervisor) Close() {
	if !s.closed.CompareAndSwap(false, true) {
		<-s.stopped
		return
	}
	close(s.stop)
	<-s.stopped
}

// run is the supervisor's only goroutine. It owns cur, state, pendingAcq, and
// the backoff timer. Outside callers communicate via cmds.
func (s *SessionSupervisor) run() {
	defer close(s.stopped)

	var (
		cur          *Session
		backoffTimer *time.Timer
		backoffC     <-chan time.Time
		doneC        <-chan struct{}
		pending      []supAcquireReq
		lastErr      string
		startedAt    time.Time
	)

	drainPending := func(sess *Session, err error) {
		for _, p := range pending {
			p.reply <- supAcquireReply{sess: sess, err: err}
		}
		pending = nil
	}

	setState := func(next SupervisorState) {
		prev := SupervisorState(s.state.Swap(int32(next)))
		if prev != next {
			SupervisorStateGauge.WithLabelValues(s.opts.Serial, prev.String()).Set(0)
			SupervisorStateGauge.WithLabelValues(s.opts.Serial, next.String()).Set(1)
		}
	}

	// tryStart attempts to open a Session. On success: cur set, state=Running,
	// doneC armed, pending drained. On failure: policy.NextDelay decides
	// whether to backoff or open the circuit.
	var tryStart func()
	tryStart = func() {
		setState(SupStarting)
		start := time.Now()
		sess, err := s.opts.OpenFunc(context.Background(), s.opts.SessionOpts)
		elapsed := time.Since(start).Seconds()
		if err != nil {
			SessionOpenSeconds.WithLabelValues(s.opts.Serial, "error").Observe(elapsed)
			lastErr = err.Error()
			cur = nil
			d, open := s.opts.Policy.NextDelay()
			if open {
				setState(SupFailed)
				drainPending(nil, ErrSessionFailed)
				return
			}
			setState(SupBackoff)
			backoffTimer = time.NewTimer(d)
			backoffC = backoffTimer.C
			return
		}
		SessionOpenSeconds.WithLabelValues(s.opts.Serial, "success").Observe(elapsed)
		startedAt = time.Now()
		lastErr = ""
		cur = sess
		setState(SupRunning)
		doneC = sess.Done()
		drainPending(sess, nil)
	}

	for {
		select {
		case <-s.stop:
			if backoffTimer != nil {
				backoffTimer.Stop()
			}
			if cur != nil {
				cur.Close()
			}
			drainPending(nil, ErrManagerClosed)
			setState(SupClosed)
			return

		case raw := <-s.cmds:
			switch c := raw.(type) {
			case supAcquireReq:
				switch SupervisorState(s.state.Load()) {
				case SupRunning:
					c.reply <- supAcquireReply{sess: cur, err: nil}
				case SupFailed:
					c.reply <- supAcquireReply{err: ErrSessionFailed}
				case SupIdle:
					pending = append(pending, c)
					tryStart()
				case SupStarting, SupBackoff:
					pending = append(pending, c)
				case SupClosed:
					c.reply <- supAcquireReply{err: ErrManagerClosed}
				}
			case supResetReq:
				s.opts.Policy.Reset()
				if SupervisorState(s.state.Load()) == SupFailed {
					setState(SupIdle)
				}
				close(c.done)
			case supCurrentReq:
				if SupervisorState(s.state.Load()) == SupRunning {
					c.reply <- cur
				} else {
					c.reply <- nil
				}
			case supSnapshotReq:
				snap := SupervisorSnapshot{
					Serial:    s.opts.Serial,
					State:     SupervisorState(s.state.Load()).String(),
					Attempts:  s.opts.Policy.Attempts(),
					LastError: lastErr,
					StartedAt: startedAt,
				}
				if cur != nil {
					snap.SessionAlive = true
					snap.Width = cur.Width()
					snap.Height = cur.Height()
				}
				c.reply <- snap
			}

		case <-backoffC:
			backoffC = nil
			backoffTimer = nil
			tryStart()

		case <-doneC:
			// Current session died. Compute backoff, schedule restart only if
			// there's pending demand or after a delay.
			PumpRestartsTotal.WithLabelValues(s.opts.Serial).Inc()
			doneC = nil
			cur = nil
			d, open := s.opts.Policy.NextDelay()
			if open {
				setState(SupFailed)
				drainPending(nil, ErrSessionFailed)
				continue
			}
			setState(SupBackoff)
			backoffTimer = time.NewTimer(d)
			backoffC = backoffTimer.C
		}
	}
}
