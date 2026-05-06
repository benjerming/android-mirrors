package scrcpy

import "testing"

func TestSessionState_String(t *testing.T) {
	cases := map[SessionState]string{
		StateStarting: "starting",
		StateRunning:  "running",
		StateClosing:  "closing",
		StateDead:     "dead",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("State(%d).String()=%q, want %q", s, got, want)
		}
	}
}

func TestSessionState_CanTransition(t *testing.T) {
	cases := []struct {
		from, to SessionState
		ok       bool
	}{
		{StateStarting, StateRunning, true},
		{StateStarting, StateDead, true},
		{StateRunning, StateDead, true},
		{StateRunning, StateClosing, true},
		{StateClosing, StateDead, true},
		{StateClosing, StateRunning, false},
		{StateClosing, StateStarting, false},
		{StateDead, StateRunning, false},
		{StateDead, StateStarting, false},
		{StateDead, StateClosing, false},
		{StateRunning, StateStarting, false},
	}
	for _, c := range cases {
		if got := c.from.CanTransition(c.to); got != c.ok {
			t.Errorf("%s -> %s: got %v, want %v", c.from, c.to, got, c.ok)
		}
	}
}
