package metrics

import (
    "time"
    "github.com/prometheus/client_golang/prometheus"
)

var (
    businessCreated = prometheus.NewCounterVec(
        prometheus.CounterOpts{Namespace: "kubeop", Subsystem: "business", Name: "created_total", Help: "Business objects created"},
        []string{"kind"},
    )
    webhookFailures = prometheus.NewCounter(
        prometheus.CounterOpts{Namespace: "kubeop", Subsystem: "webhook", Name: "failures_total", Help: "Webhook send failures"},
    )
    webhookLatency = prometheus.NewSummary(
        prometheus.SummaryOpts{Namespace: "kubeop", Subsystem: "webhook", Name: "latency_seconds", Help: "Webhook latency"},
    )
    webhookEvents = prometheus.NewCounterVec(
        prometheus.CounterOpts{Namespace: "kubeop", Subsystem: "webhook", Name: "events_total", Help: "Webhook outcomes by event"},
        []string{"event", "outcome"},
    )
    invoiceLines = prometheus.NewCounter(
        prometheus.CounterOpts{Namespace: "kubeop", Subsystem: "billing", Name: "invoice_lines_total", Help: "Invoice lines exported"},
    )
    dbLatency = prometheus.NewSummaryVec(
        prometheus.SummaryOpts{Namespace: "kubeop", Subsystem: "db", Name: "latency_seconds", Help: "DB operation latency"},
        []string{"op"},
    )
)

func init() {
    prometheus.MustRegister(businessCreated, webhookFailures, webhookLatency, webhookEvents, invoiceLines, dbLatency)
}

func IncCreated(kind string) { businessCreated.WithLabelValues(kind).Inc() }
func IncWebhookFailure() { webhookFailures.Inc() }
func ObserveWebhookLatency(d time.Duration) { webhookLatency.Observe(d.Seconds()) }
func IncWebhookEvent(event, outcome string) { webhookEvents.WithLabelValues(event, outcome).Inc() }
func AddInvoiceLines(n int) { for i := 0; i < n; i++ { invoiceLines.Inc() } }
func ObserveDB(op string, d time.Duration) { dbLatency.WithLabelValues(op).Observe(d.Seconds()) }
