package server

import (
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
	"github.com/user/llmrouter/pkg/config"
)

// WatchConfig starts a watcher for the configuration file.
func (s *Server) WatchConfig(path string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	// Initial load
	if err := s.reloadConfig(path); err != nil {
		log.Error().Err(err).Msg("failed initial config load")
	}

	go func() {
		defer func() {
			_ = watcher.Close()
		}()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				// K8s ConfigMap updates often involve re-linking symlinks,
				// so we check for WRITE or CREATE (or sometimes REMOVE/RENAME followed by CREATE)
				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
					log.Info().Str("file", event.Name).Msg("config file changed, reloading...")
					// Small delay to ensure file is fully written/stable
					time.Sleep(100 * time.Millisecond)
					if err := s.reloadConfig(path); err != nil {
						log.Error().Err(err).Msg("failed to reload config")
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Error().Err(err).Msg("config watcher error")
			}
		}
	}()

	// Watch the directory instead of the file to handle K8s symlink swaps
	return watcher.Add(filepath.Dir(path))
}

func (s *Server) reloadConfig(path string) error {
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}

	s.Registry.UpdateConfig(cfg)
	return nil
}
