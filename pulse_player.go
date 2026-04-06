package notify

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jfreymuth/pulse"
	"github.com/jfreymuth/pulse/proto"
)

type player interface {
	ensureConnected() error
	play(entry *SoundEntry, cfg *Config) error
	Close()
}

type pulsePlayer struct {
	mu     sync.Mutex
	client *pulse.Client
	cfg    *Config
}

func newPulsePlayer(cfg *Config) *pulsePlayer {
	return &pulsePlayer{cfg: cfg}
}

func (p *pulsePlayer) ensureConnected() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ensureConnectedLocked()
}

// ensureConnectedLocked must be called with p.mu held.
func (p *pulsePlayer) ensureConnectedLocked() error {
	if p.client != nil {
		if _, err := p.client.DefaultSink(); err == nil {
			return nil
		}
		p.client.Close()
		p.client = nil
	}

	opts := []pulse.ClientOption{
		pulse.ClientApplicationName("odio-notify"),
	}
	if p.cfg.PulseServer != "" {
		opts = append(opts, pulse.ClientServerString(p.cfg.PulseServer))
	}

	client, err := pulse.NewClient(opts...)
	if err != nil {
		return fmt.Errorf("pulse connect: %w", err)
	}
	p.client = client
	return nil
}

func (p *pulsePlayer) play(entry *SoundEntry, cfg *Config) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.ensureConnectedLocked(); err != nil {
		return err
	}

	reader := pulse.NewReader(
		bytes.NewReader(entry.PCM),
		proto.FormatInt16LE,
	)

	stream, err := p.client.NewPlayback(
		reader,
		pulse.PlaybackStereo,
		pulse.PlaybackSampleRate(cfg.SampleRate),
		pulse.PlaybackLatency(float64(cfg.LatencyMs)/1000.0),
		pulse.PlaybackMediaName("odio-notify"),
		pulse.PlaybackRawOption(func(req *proto.CreatePlaybackStream) {
			if req.Properties == nil {
				req.Properties = proto.PropList{}
			}
			req.Properties["media.role"] = proto.PropListString("event")
		}),
	)
	if err != nil {
		return fmt.Errorf("pulse playback: %w", err)
	}

	stream.Start()
	stream.Drain()

	if stream.Underflow() {
		log.Printf("notify: audio underflow detected")
	}

	stream.Close()
	return nil
}

func (p *pulsePlayer) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.client != nil {
		p.client.Close()
		p.client = nil
	}
}

type noopPlayer struct{}

func (noopPlayer) ensureConnected() error              { return nil }
func (noopPlayer) play(_ *SoundEntry, _ *Config) error { return nil }
func (noopPlayer) Close()                              {}

// waitReady retries ensureConnected until success or context cancellation.
func waitReady(ctx context.Context, p player, cfg *Config) error {
	ctx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()

	for {
		if err := p.ensureConnected(); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("notify: PA not ready after %v: %w", cfg.ConnectTimeout, ctx.Err())
		case <-time.After(cfg.ConnectRetryInterval):
		}
	}
}
