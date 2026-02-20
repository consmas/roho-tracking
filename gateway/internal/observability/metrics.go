package observability

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	ActiveConnections prometheus.Gauge
	FramesReceived    *prometheus.CounterVec
	FramesPublished   *prometheus.CounterVec
	CommandsHandled   *prometheus.CounterVec
	SendQueueDrops    prometheus.Counter
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		ActiveConnections: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gateway_active_connections",
			Help: "Current number of active TCP connections",
		}),
		FramesReceived: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gateway_frames_received_total",
			Help: "Total parsed frames received",
		}, []string{"event_type"}),
		FramesPublished: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gateway_frames_published_total",
			Help: "Total events published to Redis streams",
		}, []string{"event_type", "status"}),
		CommandsHandled: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gateway_commands_handled_total",
			Help: "Total command messages handled",
		}, []string{"status"}),
		SendQueueDrops: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gateway_send_queue_drops_total",
			Help: "Number of messages dropped because send queue was full",
		}),
	}
	reg.MustRegister(
		m.ActiveConnections,
		m.FramesReceived,
		m.FramesPublished,
		m.CommandsHandled,
		m.SendQueueDrops,
	)
	return m
}
