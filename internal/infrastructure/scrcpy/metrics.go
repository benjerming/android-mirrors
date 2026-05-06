package scrcpy

import "github.com/prometheus/client_golang/prometheus"

// Prometheus metrics for the scrcpy subsystem.
//
// Naming follows snake_case + unit suffix convention. Labels are kept low-
// cardinality (serial is bounded by attached device count; user IDs MUST
// NOT be added as labels — that would explode cardinality).

var (
	PumpRestartsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "scrcpy_pump_restarts_total",
		Help: "Total pump/session restarts triggered by SessionSupervisor.",
	}, []string{"serial"})

	SupervisorStateGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "scrcpy_supervisor_state",
		Help: "1 if supervisor is in this state, else 0. Exactly one state per serial is 1.",
	}, []string{"serial", "state"})

	SubscribersGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "scrcpy_subscribers",
		Help: "Current number of frame subscribers per serial.",
	}, []string{"serial"})

	FramesDroppedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "scrcpy_frames_dropped_total",
		Help: "Frames dropped due to slow subscriber.",
	}, []string{"serial"})

	SessionOpenSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "scrcpy_session_open_seconds",
		Help:    "Wall-clock latency of OpenSession (adb push + reverse + accept + meta).",
		Buckets: prometheus.DefBuckets,
	}, []string{"serial", "outcome"})
)

// RegisterMetrics registers scrcpy metrics on the given registerer.
// Bootstrap calls this once with prometheus.DefaultRegisterer.
func RegisterMetrics(r prometheus.Registerer) error {
	for _, c := range []prometheus.Collector{
		PumpRestartsTotal, SupervisorStateGauge, SubscribersGauge,
		FramesDroppedTotal, SessionOpenSeconds,
	} {
		if err := r.Register(c); err != nil {
			return err
		}
	}
	return nil
}
