package router

import (
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/user/llmrouter/pkg/config"
	"github.com/user/llmrouter/pkg/provider"
	"github.com/user/llmrouter/pkg/provider/gemini"
	"github.com/user/llmrouter/pkg/provider/groq"
	"github.com/user/llmrouter/pkg/provider/mock"
	"github.com/user/llmrouter/pkg/provider/openai"
)

// RouterRegistry manages multiple Router instances, one per agent.
type RouterRegistry struct {
	routers map[string]*Router
	mu      sync.RWMutex
}

func NewRouterRegistry() *RouterRegistry {
	return &RouterRegistry{
		routers: make(map[string]*Router),
	}
}

func (rr *RouterRegistry) GetRouter(agentID string) *Router {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	r, ok := rr.routers[agentID]
	if !ok {
		// Fallback to default if agentID not found
		return rr.routers["default"]
	}
	return r
}

func (rr *RouterRegistry) UpdateConfig(cfg *config.Config) {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	newRouters := make(map[string]*Router)

	for agentID, agentCfg := range cfg.Agents {
		var providers []*ProviderWithMetadata
		for _, pc := range agentCfg.Providers {
			var p provider.Provider
			switch pc.Type {
			case "openai":
				p = openai.NewOpenAIProvider(pc.APIKey, pc.BaseURL)
			case "groq":
				p = groq.NewGroqProvider(pc.APIKey, pc.BaseURL)
			case "gemini":
				p = gemini.NewGeminiProvider(pc.Name, pc.APIKey, pc.BaseURL)
			case "mock":
				p = mock.NewMockProvider(pc.Name, 0, nil)
			default:
				log.Warn().Str("type", pc.Type).Str("agent", agentID).Msg("unknown provider type, skipping")
				continue
			}

			providers = append(providers, &ProviderWithMetadata{
				Provider:        p,
				PromptPrice:     pc.PromptPrice,
				CompletionPrice: pc.CompletionPrice,
			})
		}

		newRouters[agentID] = NewRouter(
			providers,
			agentCfg.Routing.Strategy,
			agentCfg.Routing.Failover,
			agentCfg.Routing.Retries,
		).WithAgentID(agentID)
		log.Info().Str("agent", agentID).Int("providers", len(providers)).Msg("router initialized for agent")
	}

	rr.routers = newRouters
	log.Info().Int("agents", len(newRouters)).Msg("router registry updated")
}
