# Odio Notify

> Audio notification library and CLI for the odio ecosystem

[![CI](https://github.com/b0bbywan/go-odio-notify/actions/workflows/ci.yml/badge.svg)](https://github.com/b0bbywan/go-odio-notify/actions/workflows/ci.yml)
[![Build](https://github.com/b0bbywan/go-odio-notify/actions/workflows/build.yml/badge.svg)](https://github.com/b0bbywan/go-odio-notify/actions/workflows/build.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/b0bbywan/go-odio-notify)](https://goreportcard.com/report/github.com/b0bbywan/go-odio-notify)
[![Go Reference](https://pkg.go.dev/badge/github.com/b0bbywan/go-odio-notify.svg)](https://pkg.go.dev/github.com/b0bbywan/go-odio-notify)
[![GitHub Sponsors](https://img.shields.io/github/sponsors/b0bbywan?label=Sponsor&logo=GitHub)](https://github.com/sponsors/b0bbywan)

> Part of the [odio](https://odio.love/) project.

Pure Go audio notification library using PulseAudio (no CGO). Plays short event sounds (CD insert, Bluetooth connect, errors, etc.) with lazy connection, automatic reconnection, and embedded sounds.

Used by [go-mpd-discplayer](https://github.com/b0bbywan/go-mpd-discplayer) and [go-odio-api](https://github.com/b0bbywan/go-odio-api).

## Features

- **PulseAudio backend** via [jfreymuth/pulse](https://github.com/jfreymuth/pulse) (pure Go, no CGO)
- **Lazy connection with retry** — works even if PulseAudio isn't up yet at boot
- **`media.role=event`** on every stream — PipeWire/PA treats sounds as system events, not music
- **Embedded sounds** via `embed.FS` with filesystem override
- **Thread-safe** serialized playback queue
- **Non-blocking `Play()`** and blocking `PlaySync()`

## Library Usage

```go
import notify "github.com/b0bbywan/go-odio-notify"

// Create a notifier (connection is lazy, does not block)
n, err := notify.New(notify.Config{
    Backend: "pulse",
})
if err != nil {
    log.Fatal(err)
}
defer n.Close()

// Optionally wait for PulseAudio to be ready
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
if err := n.WaitReady(ctx); err != nil {
    log.Printf("starting without audio: %v", err)
}

// Non-blocking
n.Play(notify.EventStart)
n.Play(notify.EventConnect)

// Blocking (waits for playback to finish)
n.PlaySync(context.Background(), notify.EventError)
```

### Available Events

| Event | Description |
|---|---|
| `EventStart` | Application startup |
| `EventStop` | Application shutdown |
| `EventConnect` | Device connected (CD, USB, Bluetooth, etc.) |
| `EventDisconnect` | Device disconnected |
| `EventError` | Error notification |

### Configuration

```go
notify.Config{
    Backend:              "pulse", // "pulse" (default) or "none"
    PulseServer:          "",      // PA server string (empty = default)
    SoundsDir:            "",      // override embedded sounds from filesystem
    SampleRate:           44100,   // default: 44100
    LatencyMs:            100,     // default: 100
    ConnectTimeout:       5 * time.Second, // default: 5s
    ConnectRetryInterval: time.Second,     // default: 1s
}
```

All fields are optional — zero values use sensible defaults.

> **Tip:** systemd services should depend on `sound.target` (`After=sound.target`)
> so PulseAudio is already up when the service starts. The default 5s timeout
> is a safety net, not a substitute for proper ordering.

## CLI

A standalone CLI is included for testing and scripting.

### Install

```bash
go install github.com/b0bbywan/go-odio-notify/cmd/odio-notify@latest
```

Pre-built binaries (amd64, arm64, armv7, armv6) are available on each [release](https://github.com/b0bbywan/go-odio-notify/releases).

### Usage

```bash
# Play a sound
odio-notify play start
odio-notify play connect disconnect

# List available events
odio-notify list

# Options
odio-notify --sounds-dir /path/to/custom play error
odio-notify --pulse-server tcp:192.168.1.10 play bluetooth
odio-notify --timeout 10s play start
```

## Sound Format

PCM raw, signed 16-bit LE, stereo, 44100 Hz. Target duration: 0.5-1.5s.

Generate a placeholder:
```bash
ffmpeg -f lavfi -i "sine=frequency=880:duration=0.5" \
  -f s16le -acodec pcm_s16le -ac 2 -ar 44100 start.pcm
```

## Development

### Prerequisites

- Go 1.24 or higher

### Building

```bash
# Library
go build ./...

# CLI
go build ./cmd/odio-notify/
```

### Testing

```bash
go test -race ./...
go test -cover ./...
```

## Dependencies

- [jfreymuth/pulse](https://github.com/jfreymuth/pulse) — pure Go PulseAudio client

## License

BSD 2-Clause License — see the LICENSE file for details.
