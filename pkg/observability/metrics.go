package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RequestDuration tracks the latency of LLM requests by agent, provider and model.
	RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "llm_router_request_duration_seconds",
		Help:    "Duration of LLM requests in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"agent_id", "provider", "model", "status"})

	// TokenUsage tracks the number of tokens consumed by agent, provider and model.
	TokenUsage = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "llm_router_token_usage_total",
		Help: "Total number of tokens used.",
	}, []string{"agent_id", "provider", "model", "type"}) // type: prompt, completion, total

	// ProviderHealth tracks the availability of providers.
	ProviderHealth = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "llm_router_provider_health",
		Help: "Health status of the provider (1 for healthy, 0 for unhealthy).",
	}, []string{"provider"})
)
