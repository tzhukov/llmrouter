package config

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// RemoteProvider handles streaming configuration updates from the Control Plane.
type RemoteProvider struct {
	url string
}

func NewRemoteProvider(url string) *RemoteProvider {
	return &RemoteProvider{url: url}
}

// Watch starts a long-lived SSE connection and sends updates to the provided channel.
func (p *RemoteProvider) Watch(ctx context.Context, updateCh chan<- map[string]AgentConfig) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			err := p.subscribe(ctx, updateCh)
			if err != nil {
				log.Error().Err(err).Msg("remote provider connection lost, retrying in 5s...")
				time.Sleep(5 * time.Second)
			}
		}
	}
}

func (p *RemoteProvider) subscribe(ctx context.Context, updateCh chan<- map[string]AgentConfig) error {
	req, err := http.NewRequestWithContext(ctx, "GET", p.url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "text/event-stream")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}

		data := bytes.TrimPrefix(line, []byte("data: "))
		var agents map[string]AgentConfig
		if err := json.Unmarshal(data, &agents); err != nil {
			log.Error().Err(err).Msg("failed to unmarshal remote config")
			continue
		}

		log.Info().Int("agents", len(agents)).Msg("received remote config update")
		updateCh <- agents
	}

	return scanner.Err()
}
