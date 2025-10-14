package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	watcherEventsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kubeop_watcher_events_total",
			Help: "Number of Kubernetes objects enqueued for delivery to kubeOP, labelled by kind and event type.",
		},
		[]string{"kind", "event_type"},
	)

	watcherEventsDropped = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kubeop_watcher_events_dropped_total",
			Help: "Number of Kubernetes objects ignored by the watcher due to filtering or deduplication.",
		},
		[]string{"reason"},
	)

	watcherBatchesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kubeop_watcher_batches_total",
			Help: "Number of batches sent to kubeOP, labelled by result (success|failure).",
		},
		[]string{"result"},
	)

	watcherQueueGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "kubeop_watcher_queue_depth",
			Help: "Current number of events waiting to be delivered to kubeOP.",
		},
	)

	watcherLastSuccessfulPush = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "kubeop_watcher_last_successful_push_timestamp",
			Help: "Unix timestamp of the last successful POST to kubeOP.",
		},
	)
)

func init() {
	prometheus.MustRegister(watcherEventsTotal)
	prometheus.MustRegister(watcherEventsDropped)
	prometheus.MustRegister(watcherBatchesTotal)
	prometheus.MustRegister(watcherQueueGauge)
	prometheus.MustRegister(watcherLastSuccessfulPush)
}

// ObserveEnqueue increments the events counter for the provided kind and event
// type.
func ObserveEnqueue(kind, eventType string) {
	watcherEventsTotal.WithLabelValues(kind, eventType).Inc()
}

// ObserveDrop increments the drop counter with the provided reason.
func ObserveDrop(reason string) {
	watcherEventsDropped.WithLabelValues(reason).Inc()
}

// ObserveBatch records the result of a batch send attempt.
func ObserveBatch(result string) {
	watcherBatchesTotal.WithLabelValues(result).Inc()
}

// SetQueueDepth updates the queue depth gauge.
func SetQueueDepth(depth int) {
	watcherQueueGauge.Set(float64(depth))
}

// SetLastSuccessfulPush stores the timestamp of the last successful request.
func SetLastSuccessfulPush(ts int64) {
	watcherLastSuccessfulPush.Set(float64(ts))
}
