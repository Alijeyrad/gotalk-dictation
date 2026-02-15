# GoTalk Dictation

A fast, native Linux speech-to-text app. Press a hotkey anywhere, speak, and the transcribed text is typed at your cursor.

[![CI](https://github.com/Alijeyrad/gotalk-dictation/actions/workflows/ci.yml/badge.svg)](https://github.com/Alijeyrad/gotalk-dictation/actions/workflows/ci.yml)
[![Latest release](https://img.shields.io/github/v/release/Alijeyrad/gotalk-dictation)](https://github.com/Alijeyrad/gotalk-dictation/releases/latest)
[![Go version](https://img.shields.io/github/go-mod/go-version/Alijeyrad/gotalk-dictation)](go.mod)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## Features

- **System-wide dictation** — works in any application
- **Global hotkey** — default `Alt+D`, fully rebindable live from Settings
- **Push-to-talk mode** — hold the hotkey to record, release to submit; or use toggle mode (press once to start, again to cancel)
- **Undo last dictation** — dedicated undo hotkey (default `Alt+Z`) backspaces exactly what was typed
- **Visual indicator** — X11 overlay shows state and a preview of the transcribed text
- **Voice Activity Detection** — auto-stops after silence; configurable sensitivity and silence duration
- **Fast typing** — short text typed directly via xdotool; longer text pasted via clipboard for near-instant insertion
- **Punctuation commands** — say "period", "comma", "question mark", etc.
- **25 languages** — including regional variants; see full list in Settings
- **No ffmpeg** — pure Go FLAC encoder, no external audio tools needed
- **Two API modes** — free public Google API (no account needed) or Google Cloud Speech API

## Installation

### From a release (recommended)

Download the latest binary from the [Releases](https://github.com/Alijeyrad/gotalk-dictation/releases/latest) page:

```bash
# Install runtime deps first (see below)
curl -Lo gotalk-dictation.tar.gz \
  https://github.com/Alijeyrad/gotalk-dictation/releases/latest/download/gotalk-dictation_VERSION_linux_amd64.tar.gz
tar xf gotalk-dictation.tar.gz
sudo install -m755 gotalk-dictation /usr/local/bin/
sudo install -m644 com.alijeyrad.GoTalkDictation.desktop /usr/share/applications/
```

### Via Flatpak

> Flatpak packaging is in progress. Instructions will be added once the app is submitted to Flathub.
> See [`packaging/flatpak/`](packaging/flatpak/) for build instructions.

### From source

```bash
git clone https://github.com/Alijeyrad/gotalk-dictation.git
cd gotalk-dictation
make deps      # install system build deps (dnf/apt/pacman)
make install   # builds and installs to /usr/local/bin + system .desktop file
make autostart # optional: start at login
```

## Prerequisites

### Runtime dependencies

```bash
# Fedora/RHEL
sudo dnf install -y alsa-utils xdotool xclip

# Ubuntu/Debian
sudo apt install -y alsa-utils xdotool xclip

# Arch
sudo pacman -S alsa-utils xdotool xclip
```

- `arecord` (from `alsa-utils`) captures the microphone.
- `xdotool` types short transcripts; `xclip` pastes longer ones (≥50 chars) for near-instant insertion.

### Build dependencies (source only)

```bash
# Fedora/RHEL
sudo dnf install -y gcc libX11-devel libXcursor-devel libXrandr-devel \
  libXinerama-devel libXi-devel mesa-libGL-devel

# Ubuntu/Debian
sudo apt install -y gcc libx11-dev libxcursor-dev libxrandr-dev \
  libxinerama-dev libxi-dev libgl1-mesa-dev

# Arch
sudo pacman -S gcc libx11 libxcursor libxrandr libxinerama libxi mesa
```

## Usage

1. Run `gotalk-dictation` — it appears in the system tray.
2. Press **Alt+D** (or your configured hotkey) to start listening.
3. Speak. The floating indicator shows the current state.
4. Text is typed at the cursor when you stop speaking (or when you release the key in push-to-talk mode).

Press the hotkey again while listening to cancel. Press **Alt+Z** to undo the last dictation.

### Punctuation commands

| Say               | Gets typed |
| ----------------- | ---------- |
| period            | `.`        |
| comma             | `,`        |
| question mark     | `?`        |
| exclamation mark  | `!`        |
| colon             | `:`        |
| semicolon         | `;`        |
| new line          | `↵`        |
| new paragraph     | `↵↵`       |
| open parenthesis  | `(`        |
| close parenthesis | `)`        |
| dash / hyphen     | `-`        |
| ellipsis          | `...`      |

## Settings

Open **Settings** from the tray icon. All changes apply immediately — no restart needed.

| Setting                     | Description                                               |
| --------------------------- | --------------------------------------------------------- |
| Language                    | Speech recognition language (25 languages + variants)    |
| Custom API key              | Override the built-in Chromium key for the free API       |
| Use Google Cloud Speech API | Switch to the Cloud API (requires credentials)            |
| Silence end                 | How long a pause ends the phrase (~62 ms per chunk)       |
| Sensitivity                 | RMS threshold multiplier — lower picks up quieter voices  |
| Hotkey                      | Click and press any modifier+key combination              |
| Undo hotkey                 | Hotkey to backspace the last dictated text                |
| Push-to-talk hotkey         | Hold key to record, release to submit                     |
| Max duration                | Hard timeout for a single dictation session               |
| Add punctuation             | Enable spoken punctuation commands                        |

### Google Cloud Speech API (optional)

For higher accuracy, enable the Cloud API and set credentials:

```bash
export GOOGLE_APPLICATION_CREDENTIALS="/path/to/service-account-key.json"
# or: gcloud auth application-default login
```

Without credentials, the free public endpoint is used — no account needed.

## Configuration file

`~/.config/gotalk-dictation/config.json` is written automatically by the Settings window.

```json
{
  "hotkey": "Alt-d",
  "ptt_hotkey": "",
  "undo_hotkey": "Alt-z",
  "language": "en-US",
  "timeout": 60,
  "silence_chunks": 12,
  "sensitivity": 2.5,
  "api_key": "",
  "use_advanced_api": false,
  "enable_punctuation": true
}
```

## Project structure

```
gotalk-dictation/
├── main.go              — app struct, main(), callbacks
├── hotkeys.go           — hotkey binding and rebinding helpers
├── dictation.go         — dictation lifecycle (start/stop/toggle/undo)
└── internal/
    ├── audio/           — mic capture via arecord
    ├── config/          — load/save ~/.config/gotalk-dictation/config.json
    ├── hotkey/          — global X11 key grab (toggle + push-to-talk)
    ├── speech/          — VAD + free/cloud API + pure Go FLAC encoder
    ├── typing/          — xdotool/clipboard text insertion, punctuation, undo
    ├── ui/              — Fyne system tray, settings window, X11 overlay popup
    └── version/         — build-time version info (injected via ldflags)
```

## Contributing

1. Fork the repo and create a branch
2. `make fmt && make vet && make test` before opening a PR
3. Keep X11-specific code behind build tags where applicable
4. PRs welcome for bug fixes, language additions, and packaging improvements

## Roadmap

- **Segmented dictation** — send audio to the API on natural pauses so text appears clause-by-clause while speaking
- **Streaming dictation** — real-time interim results typed as you speak (Google Cloud Speech API only)
- **AUR package** — for Arch Linux users
- **Flathub** — once Flatpak manifest is complete

## License

MIT — see [LICENSE](LICENSE).

---

### 👤 Ali Julaee Rad

[![GitHub followers](https://img.shields.io/github/followers/alijeyrad?label=Follow&style=social)](https://github.com/alijeyrad)

- **GitHub**: [alijeyrad](https://github.com/alijeyrad)
- **LinkedIn**: [in/ali-julaee-rad](https://www.linkedin.com/in/ali-julaee-rad/)
- **Email**: [alijrad.dev@gmail.com](mailto:alijrad.dev@gmail.com)
