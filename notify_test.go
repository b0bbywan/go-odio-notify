package notify

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// mockPlayer is a configurable player for testing error paths.
type mockPlayer struct {
	mu           sync.Mutex
	connectErr   error
	playErr      error
	connectCount int
	playCount    int
	closed       bool
}

func (m *mockPlayer) ensureConnected() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connectCount++
	return m.connectErr
}

func (m *mockPlayer) play(_ *SoundEntry, _ *Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.connectErr != nil {
		return m.connectErr
	}
	m.playCount++
	return m.playErr
}

func (m *mockPlayer) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
}

func newTestNotifier(t *testing.T, p player) *Notifier {
	t.Helper()
	cache, err := NewSoundCache("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg := Config{Backend: "none"}
	n := &Notifier{
		player: p,
		cache:  cache,
		cfg:    &cfg,
		queue:  make(chan SoundEvent, 8),
		done:   make(chan struct{}),
	}
	go n.processQueue()
	return n
}

func TestNewNotifierNone(t *testing.T) {
	n, err := New(Config{Backend: "none"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer n.Close()

	n.Play(EventStart)
	if err := n.PlaySync(context.Background(), EventError); err != nil {
		t.Fatalf("PlaySync on noop should not fail: %v", err)
	}
}

func TestNewNotifierDefaultBackend(t *testing.T) {
	// Empty backend defaults to "pulse", should not error on New
	n, err := New(Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n.Close()
}

func TestNewNotifierInvalidBackend(t *testing.T) {
	_, err := New(Config{Backend: "invalid"})
	if err == nil {
		t.Fatal("expected error for invalid backend")
	}
}

func TestSoundCacheEmbed(t *testing.T) {
	sc, err := NewSoundCache("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, event := range AllEvents {
		entry, err := sc.Get(event)
		if err != nil {
			t.Errorf("missing embedded sound %s: %v", event, err)
			continue
		}
		if len(entry.PCM) == 0 {
			t.Errorf("empty PCM data for %s", event)
		}
	}
}

func TestSoundCacheOverride(t *testing.T) {
	sc, err := NewSoundCache("testdata")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, event := range AllEvents {
		if _, err := sc.Get(event); err != nil {
			t.Errorf("sound %s should fallback to embed: %v", event, err)
		}
	}
}

func TestSoundCacheOverrideExists(t *testing.T) {
	sc, err := NewSoundCache("sounds")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, event := range AllEvents {
		entry, err := sc.Get(event)
		if err != nil {
			t.Errorf("missing sound %s: %v", event, err)
			continue
		}
		if len(entry.PCM) == 0 {
			t.Errorf("empty PCM for %s", event)
		}
	}
}

func TestSoundCacheGetUnknown(t *testing.T) {
	sc, err := NewSoundCache("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := sc.Get("nonexistent"); err == nil {
		t.Fatal("expected error for unknown sound")
	}
}

func TestPlayQueueDrop(t *testing.T) {
	n, err := New(Config{Backend: "none"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer n.Close()

	for range 20 {
		n.Play(EventStart)
	}
}

func TestPlaySyncContext(t *testing.T) {
	n, err := New(Config{Backend: "none"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer n.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_ = n.PlaySync(ctx, EventStart)
}

func TestWaitReadyNoop(t *testing.T) {
	n, err := New(Config{Backend: "none"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer n.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := n.WaitReady(ctx); err != nil {
		t.Fatalf("WaitReady on noop should succeed: %v", err)
	}
}

func TestConcurrentPlay(t *testing.T) {
	n, err := New(Config{Backend: "none"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer n.Close()

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n.Play(EventStart)
		}()
	}
	wg.Wait()
}

// -- Mock player tests --

func TestPlaySyncConnectError(t *testing.T) {
	mp := &mockPlayer{connectErr: fmt.Errorf("connection refused")}
	n := newTestNotifier(t, mp)
	defer n.Close()

	err := n.PlaySync(context.Background(), EventStart)
	if err == nil {
		t.Fatal("expected error on connect failure")
	}
}

func TestPlaySyncPlayError(t *testing.T) {
	mp := &mockPlayer{playErr: fmt.Errorf("playback failed")}
	n := newTestNotifier(t, mp)
	defer n.Close()

	err := n.PlaySync(context.Background(), EventStart)
	if err == nil {
		t.Fatal("expected error on play failure")
	}
}

func TestPlaySyncUnknownEvent(t *testing.T) {
	mp := &mockPlayer{}
	n := newTestNotifier(t, mp)
	defer n.Close()

	err := n.PlaySync(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown event")
	}
}

func TestPlayQueueProcessesErrors(t *testing.T) {
	mp := &mockPlayer{playErr: fmt.Errorf("playback failed")}
	n := newTestNotifier(t, mp)

	n.Play(EventStart)
	n.Play(EventError)
	// Close waits for queue to drain — errors are logged, not fatal
	n.Close()

	mp.mu.Lock()
	defer mp.mu.Unlock()
	if mp.playCount != 2 {
		t.Errorf("expected 2 play calls, got %d", mp.playCount)
	}
}

func TestPlayQueueConnectError(t *testing.T) {
	mp := &mockPlayer{connectErr: fmt.Errorf("connection refused")}
	n := newTestNotifier(t, mp)

	n.Play(EventStart)
	n.Close()

	mp.mu.Lock()
	defer mp.mu.Unlock()
	if mp.playCount != 0 {
		t.Errorf("expected 0 play calls on connect error, got %d", mp.playCount)
	}
}

func TestWaitReadyTimeout(t *testing.T) {
	mp := &mockPlayer{connectErr: fmt.Errorf("not ready")}
	cfg := Config{
		ConnectTimeout:       100 * time.Millisecond,
		ConnectRetryInterval: 20 * time.Millisecond,
	}
	err := waitReady(context.Background(), mp, &cfg)
	if err == nil {
		t.Fatal("expected timeout error")
	}

	mp.mu.Lock()
	defer mp.mu.Unlock()
	if mp.connectCount < 2 {
		t.Errorf("expected multiple connect attempts, got %d", mp.connectCount)
	}
}

func TestWaitReadyContextCancel(t *testing.T) {
	mp := &mockPlayer{connectErr: fmt.Errorf("not ready")}
	cfg := Config{
		ConnectTimeout:       10 * time.Second,
		ConnectRetryInterval: 20 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(80 * time.Millisecond)
		cancel()
	}()

	err := waitReady(ctx, mp, &cfg)
	if err == nil {
		t.Fatal("expected context cancel error")
	}
}

func TestWaitReadyRetryThenSuccess(t *testing.T) {
	mp := &mockPlayer{connectErr: fmt.Errorf("not ready")}
	cfg := Config{
		ConnectTimeout:       time.Second,
		ConnectRetryInterval: 20 * time.Millisecond,
	}

	// Clear the error after a short delay
	go func() {
		time.Sleep(60 * time.Millisecond)
		mp.mu.Lock()
		mp.connectErr = nil
		mp.mu.Unlock()
	}()

	err := waitReady(context.Background(), mp, &cfg)
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}

	mp.mu.Lock()
	defer mp.mu.Unlock()
	if mp.connectCount < 2 {
		t.Errorf("expected multiple connect attempts, got %d", mp.connectCount)
	}
}

func TestCloseCallsPlayerClose(t *testing.T) {
	mp := &mockPlayer{}
	n := newTestNotifier(t, mp)
	n.Close()

	mp.mu.Lock()
	defer mp.mu.Unlock()
	if !mp.closed {
		t.Error("expected player.Close() to be called")
	}
}

func TestPlaySyncWithCustomConfig(t *testing.T) {
	mp := &mockPlayer{}
	cache, err := NewSoundCache("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg := Config{
		SampleRate: 48000,
		LatencyMs:  50,
	}
	n := &Notifier{
		player: mp,
		cache:  cache,
		cfg:    &cfg,
		queue:  make(chan SoundEvent, 8),
		done:   make(chan struct{}),
	}
	go n.processQueue()
	defer n.Close()

	if err := n.PlaySync(context.Background(), EventStart); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
