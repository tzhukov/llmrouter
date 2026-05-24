// Package main is the entry point for the llmrouter server.
package main

import (
	"context"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/user/llmrouter/pkg/server"
)

func main() {
	// Configure zerolog for structured JSON logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(os.Stdout)

	s := server.NewServer()

	configSource := os.Getenv("CONFIG_SOURCE")
	if configSource == "" {
		configSource = "file"
	}

	if configSource == "remote" {
		configURL := os.Getenv("CONFIG_URL")
		if configURL == "" {
			configURL = "http://llm-config-server:8081/v1/sync"
		}
		log.Info().Str("url", configURL).Msg("using remote config source")
		s.WatchRemoteConfig(context.Background(), configURL)
	} else {
		configPath := os.Getenv("CONFIG_PATH")
		if configPath == "" {
			configPath = "config.yaml"
		}

		// Create dummy config if it doesn't exist for demo/phase 1 purposes
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			dummyConfig := `
providers:
  - name: mock-1
    type: mock
`
			if err := os.WriteFile(configPath, []byte(dummyConfig), 0644); err != nil {
				log.Error().Err(err).Msg("failed to create dummy config")
			}
		}

		if err := s.WatchConfig(configPath); err != nil {
			log.Warn().Err(err).Msg("failed to start config watcher, using default/empty config")
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if err := s.Start(":" + port); err != nil {
		log.Fatal().Err(err).Msg("server failed")
	}
}
