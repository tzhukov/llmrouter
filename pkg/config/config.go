// Package config provides configuration management for the llmrouter.
package config

import (
	"os"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// Config represents the root configuration structure.
type Config struct {
	Providers []ProviderConfig       `yaml:"providers"`
	Routing   RoutingConfig          `yaml:"routing"`
	Agents    map[string]AgentConfig `yaml:"agents"`
}

// AgentConfig represents the configuration for a specific agent.
type AgentConfig struct {
	Providers []ProviderConfig `yaml:"providers"`
	Routing   RoutingConfig    `yaml:"routing"`
}

// ProviderConfig represents the configuration for an LLM provider.
type ProviderConfig struct {
	Name            string            `yaml:"name"`
	Type            string            `yaml:"type"`
	BaseURL         string            `yaml:"base_url"`
	APIKey          string            `yaml:"api_key"`
	Weight          int               `yaml:"weight"`
	PromptPrice     float64           `yaml:"prompt_price"`     // Price per 1k tokens
	CompletionPrice float64           `yaml:"completion_price"` // Price per 1k tokens
	Models          []string          `yaml:"models"`           // Supported models
	Params          map[string]string `yaml:"params"`
}

// RoutingConfig represents the routing strategy configuration.
type RoutingConfig struct {
	Strategy string `yaml:"strategy"`
	Failover bool   `yaml:"failover"`
	Retries  int    `yaml:"retries"`
}

// Load loads the configuration from a YAML file and expands environment variables.
func Load(path string) (*Config, error) {
	log.Debug().Str("path", path).Msg("loading configuration from file")
	data, err := os.ReadFile(path)
	if err != nil {
		log.Error().Err(err).Str("path", path).Msg("error reading configuration file")
		return nil, err
	}

	log.Debug().Msg("expanding environment variables in configuration")
	// Simple env var expansion: ${VAR}
	expanded := os.ExpandEnv(string(data))

	var cfg Config
	log.Debug().Msg("unmarshalling YAML configuration")
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		log.Error().Err(err).Msg("error unmarshalling YAML")
		return nil, err
	}

	// Ensure backward compatibility by moving global providers/routing to default agent if needed
	if cfg.Agents == nil {
		cfg.Agents = make(map[string]AgentConfig)
	}
	if _, ok := cfg.Agents["default"]; !ok {
		cfg.Agents["default"] = AgentConfig{
			Providers: cfg.Providers,
			Routing:   cfg.Routing,
		}
	}

	log.Info().Str("path", path).Msg("configuration loaded successfully")
	return &cfg, nil
}

// GetEnv helper to get env with fallback
func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
