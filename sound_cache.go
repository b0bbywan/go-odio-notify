package notify

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed sounds/*.pcm
var defaultSounds embed.FS

// SoundEntry holds raw PCM data for a single notification sound.
// Format: signed 16-bit LE, stereo, 44100 Hz.
type SoundEntry struct {
	PCM []byte
}

// SoundCache maps SoundEvents to their preloaded PCM data.
type SoundCache struct {
	sounds map[SoundEvent]*SoundEntry
}

// NewSoundCache loads all notification sounds into memory. If soundsDir is
// non-empty, files in that directory override the corresponding embedded
// sounds. Missing overrides fall back to the embedded default.
func NewSoundCache(soundsDir string) (*SoundCache, error) {
	sc := &SoundCache{sounds: make(map[SoundEvent]*SoundEntry)}

	for _, event := range AllEvents {
		// Filesystem override
		if soundsDir != "" {
			path := filepath.Join(soundsDir, event.Filename())
			if data, err := os.ReadFile(path); err == nil {
				sc.sounds[event] = &SoundEntry{PCM: data}
				continue
			}
		}

		// Fallback to embedded sounds
		data, err := defaultSounds.ReadFile("sounds/" + event.Filename())
		if err != nil {
			return nil, fmt.Errorf("missing embedded sound %s: %w", event.Filename(), err)
		}
		sc.sounds[event] = &SoundEntry{PCM: data}
	}

	return sc, nil
}

// Get returns the PCM data for the given event, or an error if unknown.
func (sc *SoundCache) Get(event SoundEvent) (*SoundEntry, error) {
	entry, ok := sc.sounds[event]
	if !ok {
		return nil, fmt.Errorf("sound %s not found", event)
	}
	return entry, nil
}
