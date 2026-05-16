package main

import (
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
		os.WriteFile(configPath, []byte(dummyConfig), 0644)
	}

	if err := s.WatchConfig(configPath); err != nil {
		log.Warn().Err(err).Msg("failed to start config watcher, using default/empty config")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if err := s.Start(":" + port); err != nil {
		log.Fatal().Err(err).Msg("server failed")
	}
}
