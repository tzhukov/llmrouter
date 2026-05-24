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

// Registry manages multiple Router instances, one per agent.
type Registry struct {
	routers map[string]*Router
	mu      sync.RWMutex
}

// NewRegistry creates a new Router Registry.
func NewRegistry() *Registry {
	return &Registry{
		routers: make(map[string]*Router),
	}
}

// GetRouter returns the router for the given agent ID.
func (rr *Registry) GetRouter(agentID string) *Router {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	r, ok := rr.routers[agentID]
	if !ok {
		// Fallback to default if agentID not found
		return rr.routers["default"]
	}
	return r
}

// UpdateConfig updates the registry with the given configuration.
func (rr *Registry) UpdateConfig(cfg *config.Config) {
	rr.UpdateAgents(cfg.Agents)
}

// UpdateAgents updates the registry with the given agents configuration.
func (rr *Registry) UpdateAgents(agents map[string]config.AgentConfig) {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	newRouters := make(map[string]*Router)

	for agentID, agentCfg := range agents {
		var providers []*ProviderWithMetadata
		for _, pc := range agentCfg.Providers {
			var p provider.Provider
			switch pc.Type {
			case "openai":
				p = openai.NewProvider(pc.APIKey, pc.BaseURL)
			case "groq":
				p = groq.NewProvider(pc.APIKey, pc.BaseURL)
			case "gemini":
				p = gemini.NewProvider(pc.Name, pc.APIKey, pc.BaseURL)
			case "mock":
				p = mock.NewProvider(pc.Name, 0, nil)
			default:
				log.Warn().Str("type", pc.Type).Str("agent", agentID).Msg("unknown provider type, skipping")
				continue
			}

			providers = append(providers, &ProviderWithMetadata{
				Provider:        p,
				PromptPrice:     pc.PromptPrice,
				CompletionPrice: pc.CompletionPrice,
				Models:          pc.Models,
			})

			// Inject model map if provided in params
			if pc.Type == "gemini" && len(pc.Params) > 0 {
				if gp, ok := p.(*gemini.Provider); ok {
					gp.SetModelMap(pc.Params)
				}
			}
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
