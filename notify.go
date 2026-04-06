// Package notify provides audio notification playback for the odio ecosystem.
//
// It supports PulseAudio via a pure Go client (no CGO) with lazy connection,
// automatic reconnection, embedded sounds with filesystem override, and a
// serialized playback queue safe for concurrent use.
//
// Quick start:
//
//	n, err := notify.New(notify.Config{Backend: "pulse"})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer n.Close()
//
//	n.Play(notify.EventStart)
package notify

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

const (
	defaultConnectTimeout       = 5 * time.Second
	defaultConnectRetryInterval = time.Second
)

// SoundEvent identifies a notification type.
type SoundEvent string

const (
	EventStart      SoundEvent = "start"
	EventStop       SoundEvent = "stop"
	EventConnect    SoundEvent = "connect"
	EventDisconnect SoundEvent = "disconnect"
	EventError      SoundEvent = "error"
)

// Filename returns the PCM filename for this event (e.g. "start.pcm").
func (e SoundEvent) Filename() string {
	return string(e) + ".pcm"
}

// AllEvents lists every registered SoundEvent.
var AllEvents = []SoundEvent{
	EventStart,
	EventStop,
	EventConnect,
	EventDisconnect,
	EventError,
}

// Config holds the Notifier configuration. Zero values provide sensible
// defaults: backend "pulse", sample rate 44100, latency 100ms, connect
// timeout 5s, retry interval 1s.
type Config struct {
	// Backend: "pulse" or "none"
	Backend string

	// PulseServer: PA connection string (empty = default/PULSE_SERVER)
	PulseServer string

	// SoundsDir: custom directory to override embedded sounds.
	// If empty, uses embedded sounds.
	SoundsDir string

	// SampleRate: default 44100
	SampleRate int

	// LatencyMs: target latency in ms, default 100
	LatencyMs int

	// ConnectTimeout: max timeout to wait for PA at boot, default 5s.
	// Callers using systemd should depend on sound.target so PA is
	// already up by the time the service starts.
	ConnectTimeout time.Duration

	// ConnectRetryInterval: interval between retries, default 1s
	ConnectRetryInterval time.Duration
}

// Notifier plays notification sounds via PulseAudio.
// It is safe for concurrent use. Play is non-blocking; PlaySync blocks
// until playback completes.
type Notifier struct {
	player    player
	cache     *SoundCache
	cfg       *Config
	queue     chan SoundEvent
	done      chan struct{}
	closeOnce sync.Once
}

// New creates a Notifier. It does not block: the PulseAudio connection is
// established lazily on the first Play call. If backend is "none", a no-op
// notifier is returned (useful for testing or disabling audio).
func New(cfg Config) (*Notifier, error) {
	if cfg.SampleRate == 0 {
		cfg.SampleRate = 44100
	}
	if cfg.LatencyMs == 0 {
		cfg.LatencyMs = 100
	}
	if cfg.ConnectTimeout == 0 {
		cfg.ConnectTimeout = defaultConnectTimeout
	}
	if cfg.ConnectRetryInterval == 0 {
		cfg.ConnectRetryInterval = defaultConnectRetryInterval
	}

	cache, err := NewSoundCache(cfg.SoundsDir)
	if err != nil {
		return nil, fmt.Errorf("notify: sound cache: %w", err)
	}

	var p player
	switch cfg.Backend {
	case "pulse", "":
		p = newPulsePlayer(&cfg)
	case "none":
		p = noopPlayer{}
	default:
		return nil, fmt.Errorf("notify: unsupported backend %q", cfg.Backend)
	}

	n := &Notifier{
		player: p,
		cache:  cache,
		cfg:    &cfg,
		queue:  make(chan SoundEvent, 8),
		done:   make(chan struct{}),
	}
	go n.processQueue()
	return n, nil
}

// WaitReady blocks until the audio backend is connected
// or the context is cancelled.
func (n *Notifier) WaitReady(ctx context.Context) error {
	return waitReady(ctx, n.player, n.cfg)
}

// Play enqueues a sound and returns immediately.
// If the queue is full, the event is dropped.
func (n *Notifier) Play(event SoundEvent) {
	select {
	case n.queue <- event:
	default:
		log.Printf("notify: queue full, dropping %s", event)
	}
}

// PlaySync plays a sound and blocks until playback is complete.
func (n *Notifier) PlaySync(ctx context.Context, event SoundEvent) error {
	return n.playOne(ctx, event)
}

// Close drains the queue and releases resources. Safe to call multiple times.
func (n *Notifier) Close() {
	n.closeOnce.Do(func() {
		close(n.queue)
		<-n.done
		n.player.Close()
	})
}

func (n *Notifier) processQueue() {
	defer close(n.done)
	for event := range n.queue {
		if err := n.playOne(context.Background(), event); err != nil {
			log.Printf("notify: %s: %v", event, err)
		}
	}
}

func (n *Notifier) playOne(ctx context.Context, event SoundEvent) error {
	entry, err := n.cache.Get(event)
	if err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		done <- n.player.play(entry, n.cfg)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}
