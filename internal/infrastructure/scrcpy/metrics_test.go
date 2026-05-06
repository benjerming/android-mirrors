package scrcpy

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetrics_Registration(t *testing.T) {
	reg := prometheus.NewRegistry()
	if err := RegisterMetrics(reg); err != nil {
		t.Fatalf("register: %v", err)
	}
	// Double-registration must fail (catches accidental double-register in bootstrap).
	if err := RegisterMetrics(reg); err == nil {
		t.Errorf("expected error on double-register")
	}
}

func TestMetrics_PumpRestartCounter(t *testing.T) {
	PumpRestartsTotal.Reset()
	PumpRestartsTotal.WithLabelValues("emu5554").Inc()
	PumpRestartsTotal.WithLabelValues("emu5554").Inc()
	if got := testutil.ToFloat64(PumpRestartsTotal.WithLabelValues("emu5554")); got != 2 {
		t.Errorf("got %v restarts, want 2", got)
	}
}

func TestMetrics_SubscriberGauge(t *testing.T) {
	SubscribersGauge.Reset()
	SubscribersGauge.WithLabelValues("emu5554").Inc()
	SubscribersGauge.WithLabelValues("emu5554").Inc()
	SubscribersGauge.WithLabelValues("emu5554").Dec()
	if got := testutil.ToFloat64(SubscribersGauge.WithLabelValues("emu5554")); got != 1 {
		t.Errorf("got %v subscribers, want 1", got)
	}
}

func TestMetrics_AllNamesRegistered(t *testing.T) {
	reg := prometheus.NewRegistry()
	if err := RegisterMetrics(reg); err != nil {
		t.Fatalf("register: %v", err)
	}
	expected := []string{
		"scrcpy_pump_restarts_total",
		"scrcpy_supervisor_state",
		"scrcpy_subscribers",
		"scrcpy_frames_dropped_total",
		"scrcpy_session_open_seconds",
	}
	metrics, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	have := map[string]bool{}
	for _, m := range metrics {
		have[m.GetName()] = true
	}
	for _, want := range expected {
		if !have[want] {
			// Histogram only shows up after at least one observation; touch it.
			if want == "scrcpy_session_open_seconds" {
				SessionOpenSeconds.WithLabelValues("emu5554", "success").Observe(0.001)
				continue
			}
			// Counters/gauges with labels also need a touch.
			switch want {
			case "scrcpy_pump_restarts_total":
				PumpRestartsTotal.WithLabelValues("emu5554").Inc()
			case "scrcpy_supervisor_state":
				SupervisorStateGauge.WithLabelValues("emu5554", "running").Set(1)
			case "scrcpy_subscribers":
				SubscribersGauge.WithLabelValues("emu5554").Inc()
			case "scrcpy_frames_dropped_total":
				FramesDroppedTotal.WithLabelValues("emu5554").Inc()
			}
		}
	}
	// Re-gather after touching
	metrics, _ = reg.Gather()
	have = map[string]bool{}
	for _, m := range metrics {
		have[m.GetName()] = true
	}
	for _, want := range expected {
		if !have[want] {
			t.Errorf("metric %q not registered after touch", want)
		}
	}
}
