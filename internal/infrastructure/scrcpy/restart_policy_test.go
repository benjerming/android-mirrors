package scrcpy

import (
	"testing"
	"time"
)

func TestRestartPolicy_BackoffSchedule(t *testing.T) {
	p := NewRestartPolicy(RestartPolicyOpts{
		BaseDelay:   time.Second,
		MaxDelay:    16 * time.Second,
		MaxAttempts: 5,
	})
	want := []time.Duration{
		time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
	}
	for i, w := range want {
		d, open := p.NextDelay()
		if open {
			t.Fatalf("attempt %d: circuit unexpectedly open", i+1)
		}
		if d != w {
			t.Errorf("attempt %d: delay=%s, want %s", i+1, d, w)
		}
	}
	if _, open := p.NextDelay(); !open {
		t.Errorf("expected circuit open after 5 attempts")
	}
}

func TestRestartPolicy_MaxDelayCaps(t *testing.T) {
	p := NewRestartPolicy(RestartPolicyOpts{
		BaseDelay:   time.Second,
		MaxDelay:    3 * time.Second,
		MaxAttempts: 10,
	})
	seen := map[time.Duration]bool{}
	for i := 0; i < 5; i++ {
		d, _ := p.NextDelay()
		seen[d] = true
		if d > 3*time.Second {
			t.Errorf("delay %s exceeds MaxDelay", d)
		}
	}
	if !seen[3*time.Second] {
		t.Errorf("expected to hit cap at 3s; saw %v", seen)
	}
}

func TestRestartPolicy_ResetOnSuccess(t *testing.T) {
	p := NewRestartPolicy(RestartPolicyOpts{
		BaseDelay: time.Second, MaxDelay: 16 * time.Second, MaxAttempts: 5,
	})
	_, _ = p.NextDelay()
	_, _ = p.NextDelay()
	p.Reset()
	d, _ := p.NextDelay()
	if d != time.Second {
		t.Errorf("after Reset: delay=%s, want 1s", d)
	}
	if got := p.Attempts(); got != 1 {
		t.Errorf("Attempts=%d, want 1 after one NextDelay post-Reset", got)
	}
}

func TestRestartPolicy_Defaults(t *testing.T) {
	p := NewRestartPolicy(RestartPolicyOpts{})
	d, open := p.NextDelay()
	if open {
		t.Fatalf("default config opened circuit on first call")
	}
	if d <= 0 {
		t.Errorf("default base delay should be > 0, got %s", d)
	}
}
