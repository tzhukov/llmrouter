package router

import (
	"context"
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/user/llmrouter/pkg/api"
	"github.com/user/llmrouter/pkg/observability"
	"github.com/user/llmrouter/pkg/provider"
)

var (
	ErrNoProviders = errors.New("no providers available")
)

type ProviderWithMetadata struct {
	provider.Provider
	PromptPrice    float64
	CompletionPrice float64
	AvgLatency     float64
	latencyMu      sync.RWMutex
}

func (p *ProviderWithMetadata) UpdateLatency(latency float64) {
	p.latencyMu.Lock()
	defer p.latencyMu.Unlock()
	// Simple Moving Average (alpha = 0.2)
	if p.AvgLatency == 0 {
		p.AvgLatency = latency
	} else {
		p.AvgLatency = p.AvgLatency*0.8 + latency*0.2
	}
}

func (p *ProviderWithMetadata) GetLatency() float64 {
	p.latencyMu.RLock()
	defer p.latencyMu.RUnlock()
	return p.AvgLatency
}

// Router manages a pool of providers and handles routing logic.
type Router struct {
	providers []*ProviderWithMetadata
	strategy  string
	failover  bool
	retries   int
	
	current   uint64 // for round-robin
	mu        sync.RWMutex
}

func NewRouter(providers []*ProviderWithMetadata, strategy string, failover bool, retries int) *Router {
	return &Router{
		providers: providers,
		strategy:  strategy,
		failover:  failover,
		retries:   retries,
	}
}

// UpdateProviders safely replaces the provider pool (used for hot-reload).
func (r *Router) UpdateProviders(newProviders []*ProviderWithMetadata) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers = newProviders
	atomic.StoreUint64(&r.current, 0)
	log.Info().Int("count", len(newProviders)).Msg("router providers updated")
}

func (r *Router) SetStrategy(strategy string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.strategy = strategy
}

// selectProviders orders providers according to the active routing strategy.
func (r *Router) selectProviders(providers []*ProviderWithMetadata, strategy string) []*ProviderWithMetadata {
	switch strategy {
	case "cost":
		sorted := make([]*ProviderWithMetadata, len(providers))
		copy(sorted, providers)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].PromptPrice < sorted[j].PromptPrice
		})
		return sorted
	case "latency":
		sorted := make([]*ProviderWithMetadata, len(providers))
		copy(sorted, providers)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].GetLatency() < sorted[j].GetLatency()
		})
		return sorted
	default: // round-robin
		startIdx := int(atomic.AddUint64(&r.current, 1) % uint64(len(providers)))
		sorted := make([]*ProviderWithMetadata, 0, len(providers))
		for i := 0; i < len(providers); i++ {
			sorted = append(sorted, providers[(startIdx+i)%len(providers)])
		}
		return sorted
	}
}

func (r *Router) ChatCompletion(ctx context.Context, req *api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
	r.mu.RLock()
	providers := r.providers
	strategy := r.strategy
	failover := r.failover
	retries := r.retries
	r.mu.RUnlock()

	if len(providers) == 0 {
		return nil, ErrNoProviders
	}

	sortedProviders := r.selectProviders(providers, strategy)

	maxAttempts := 1
	if failover {
		maxAttempts = len(sortedProviders)
		if retries > 0 && retries < maxAttempts {
			maxAttempts = retries + 1
		}
	}

	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		p := sortedProviders[i]
		
		logger := observability.GetLogger(ctx)
		logger.Debug().
			Str("provider", p.Name()).
			Int("attempt", i+1).
			Str("strategy", strategy).
			Float64("avg_latency", p.GetLatency()).
			Msg("routing request")
		
		start := time.Now()
		resp, err := p.ChatCompletion(ctx, req)
		duration := time.Since(start).Seconds()

		status := "success"
		if err != nil {
			status = "error"
			observability.ProviderHealth.WithLabelValues(p.Name()).Set(0)
		} else {
			observability.ProviderHealth.WithLabelValues(p.Name()).Set(1)
			p.UpdateLatency(duration)
			// Record token usage
			observability.TokenUsage.WithLabelValues(p.Name(), req.Model, "prompt").Add(float64(resp.Usage.PromptTokens))
			observability.TokenUsage.WithLabelValues(p.Name(), req.Model, "completion").Add(float64(resp.Usage.CompletionTokens))
			observability.TokenUsage.WithLabelValues(p.Name(), req.Model, "total").Add(float64(resp.Usage.TotalTokens))
		}

		observability.RequestDuration.WithLabelValues(p.Name(), req.Model, status).Observe(duration)

		if err == nil {
			return resp, nil
		}
		
		logger.Warn().Err(err).Str("provider", p.Name()).Msg("provider request failed")
		lastErr = err
		
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
	}

	return nil, lastErr
}

func (r *Router) StreamChatCompletion(ctx context.Context, req *api.ChatCompletionRequest) (<-chan *api.ChatCompletionStreamResponse, <-chan error) {
	r.mu.RLock()
	providers := r.providers
	strategy := r.strategy
	failover := r.failover
	retries := r.retries
	r.mu.RUnlock()

	respCh := make(chan *api.ChatCompletionStreamResponse)
	errCh := make(chan error, 1)

	if len(providers) == 0 {
		errCh <- ErrNoProviders
		close(respCh)
		close(errCh)
		return respCh, errCh
	}

	sortedProviders := r.selectProviders(providers, strategy)

	go func() {
		defer close(respCh)
		defer close(errCh)

		maxAttempts := 1
		if failover {
			maxAttempts = len(sortedProviders)
			if retries > 0 && retries < maxAttempts {
				maxAttempts = retries + 1
			}
		}

		var lastErr error
		for i := 0; i < maxAttempts; i++ {
			p := sortedProviders[i]
			logger := observability.GetLogger(ctx)
			logger.Debug().Str("provider", p.Name()).Int("attempt", i+1).Msg("routing streaming request")

			start := time.Now()
			pCh, pErrCh := p.StreamChatCompletion(ctx, req)

			// Wait for the first chunk or an error
			select {
			case chunk, ok := <-pCh:
				if ok {
					// We got a chunk! Streaming has started successfully.
					respCh <- chunk
					observability.ProviderHealth.WithLabelValues(p.Name()).Set(1)
					// Forward the rest of the stream
					for nextChunk := range pCh {
						respCh <- nextChunk
					}
					// Check if there was a late error
					if pErr := <-pErrCh; pErr != nil {
						logger.Warn().Err(pErr).Str("provider", p.Name()).Msg("stream failed mid-way")
					}
					observability.RequestDuration.WithLabelValues(p.Name(), req.Model, "success").Observe(time.Since(start).Seconds())
					return
				}
			case err := <-pErrCh:
				if err != nil {
					lastErr = err
					logger.Warn().Err(err).Str("provider", p.Name()).Msg("streaming provider failed to start")
					observability.ProviderHealth.WithLabelValues(p.Name()).Set(0)
					observability.RequestDuration.WithLabelValues(p.Name(), req.Model, "error").Observe(time.Since(start).Seconds())
					continue // Try next provider
				}
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			}
		}

		if lastErr != nil {
			errCh <- lastErr
		} else {
			errCh <- ErrNoProviders
		}
	}()

	return respCh, errCh
}
