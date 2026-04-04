package config

import (
	"log"

	"github.com/fsnotify/fsnotify"
)

// Watch monitors the config file at path for changes.
// When the file is saved, it reloads it and calls onChange with the new Config.
// Runs in the background — returns immediately after starting.
func Watch(path string, onChange func(*Config)) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	if err := watcher.Add(path); err != nil {
		watcher.Close()
		return err
	}

	go func() {
		defer watcher.Close()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					cfg, err := Load(path)
					if err != nil {
						log.Printf("  [ Config ] Hot-reload failed: %v", err)
						continue
					}
					log.Println("  [ Config ] centauri.yml reloaded")
					onChange(cfg)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("  [ Config ] Watcher error: %v", err)
			}
		}
	}()

	return nil
}
