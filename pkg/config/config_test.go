package config_test

import (
	"os"
	"testing"

	"github.com/user/llmrouter/pkg/config"
)

func TestLoad(t *testing.T) {
	// Create a temporary config file
	content := `
routing:
  strategy: "cost"
  failover: true
  retries: 2
providers:
  - name: test-provider
    type: openai
    api_key: "${TEST_API_KEY}"
    prompt_price: 0.01
`
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Set environment variable
	os.Setenv("TEST_API_KEY", "secret-key")
	defer os.Unsetenv("TEST_API_KEY")

	// Load config
	cfg, err := config.Load(tmpfile.Name())
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify values
	if cfg.Routing.Strategy != "cost" {
		t.Errorf("expected strategy cost, got %s", cfg.Routing.Strategy)
	}
	if cfg.Routing.Retries != 2 {
		t.Errorf("expected 2 retries, got %d", cfg.Routing.Retries)
	}

	if len(cfg.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(cfg.Providers))
	}

	p := cfg.Providers[0]
	if p.Name != "test-provider" {
		t.Errorf("expected provider name test-provider, got %s", p.Name)
	}
	if p.APIKey != "secret-key" {
		t.Errorf("expected APIKey secret-key, got %s", p.APIKey)
	}
	if p.PromptPrice != 0.01 {
		t.Errorf("expected prompt_price 0.01, got %f", p.PromptPrice)
	}
}
