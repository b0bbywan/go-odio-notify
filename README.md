  <p align="center">   
  <a href="https://odio.love"><img src="https://odio.love/logo.png" alt="odio" width="160" /></a>
  </p>
  <h1 align="center">go-odio-notify</h1>
  <p align="center"><em>Audio notification library and CLI for the odio ecosystem.</em></p>
  <p align="center">
  <a href="https://github.com/b0bbywan/go-odio-notify/releases"><img src="https://img.shields.io/github/v/release/b0bbywan/go-odio-notify?include_prereleases" alt="Release" /></a>
  <a href="https://github.com/b0bbywan/go-odio-notify/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-BSD--2--Clause-blue" alt="License" /></a>
  <a href="https://github.com/b0bbywan/go-odio-notify/actions/workflows/ci.yml"><img src="https://github.com/b0bbywan/go-odio-notify/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
  <a href="https://github.com/b0bbywan/go-odio-notify/actions/workflows/build.yml"><img src="https://github.com/b0bbywan/go-odio-notify/actions/workflows/build.yml/badge.svg" alt="Build" /></a>
  <a href="https://goreportcard.com/report/github.com/b0bbywan/go-odio-notify"><img src="https://goreportcard.com/badge/github.com/b0bbywan/go-odio-notify" alt="Go Report Card" /></a>
  <a href="https://pkg.go.dev/github.com/b0bbywan/go-odio-notify"><img src="https://pkg.go.dev/badge/github.com/b0bbywan/go-odio-notify.svg" alt="Go Reference" /></a>
  <a href="https://github.com/sponsors/b0bbywan"><img src="https://img.shields.io/github/sponsors/b0bbywan?label=Sponsor&logo=GitHub" alt="GitHub Sponsors" /></a>   
  </p>
  <p align="center">
  <a href="https://docs.odio.love/guides/audio-notifications/"><img src="https://img.shields.io/badge/Audio%20notifications-E60023" alt="Audio notifications guide" /></a>
  <a href="https://docs.odio.love/notify/overview/"><img src="https://img.shields.io/badge/Events-7C3AED" alt="Events" /></a>
  <a href="https://en.wikipedia.org/wiki/Pulse-code_modulation"><img src="https://img.shields.io/badge/PCM-4A148C" alt="PCM" /></a>                                                                        
  <a href="https://github.com/b0bbywan/odios/discussions/42"><img src="https://img.shields.io/badge/Sound%20design-odios%2342-F97316" alt="Sound design discussion odios#42" /></a>
  </p>
  <p align="center">
  Part of the <a href="https://odio.love">odio</a> project — <a href="https://docs.odio.love/notify/overview/">full documentation</a>.
  </p>
  <p align="center">
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-00ADD8?logo=go&logoColor=white" alt="Go" /></a>
  <a href="https://www.freedesktop.org/wiki/Software/PulseAudio/"><img src="https://img.shields.io/badge/PulseAudio-F47821" alt="PulseAudio" /></a>
  <a href="https://github.com/features/actions"><img src="https://img.shields.io/badge/GitHub%20Actions-2088FF?logo=githubactions&logoColor=white" alt="GitHub Actions" /></a>
  </p> 
  
  # Odio Notify

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
