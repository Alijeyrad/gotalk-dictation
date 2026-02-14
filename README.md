# GoTalk Dictation

A fast, native Linux speech-to-text app. Press a hotkey anywhere, speak, and the transcribed text is typed at your cursor.

![License](https://img.shields.io/badge/license-MIT-blue.svg)

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

`arecord` (from `alsa-utils`) captures the microphone.
`xdotool` types short transcripts; `xclip` pastes longer ones (≥50 chars) for near-instant insertion.

### Build dependencies

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

## Installation

```bash
git clone https://github.com/Alijeyrad/gotalk-dictation.git
cd gotalk-dictation
make install     # builds and installs to /usr/local/bin + system .desktop file
make autostart   # optional: start at login
```

Or build manually:

```bash
make build       # output: build/gotalk-dictation
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
| Max duration                | Hard timeout for a single dictation session               |
| Add punctuation             | Enable spoken punctuation commands                        |
| Push-to-talk                | Hold key to record, release to submit                     |

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
  "undo_hotkey": "Alt-z",
  "language": "en-US",
  "timeout": 60,
  "silence_chunks": 12,
  "sensitivity": 2.5,
  "api_key": "",
  "use_advanced_api": false,
  "enable_punctuation": true,
  "push_to_talk": false
}
```

## Project structure

```
gotalk-dictation/
├── main.go
└── internal/
    ├── audio/recorder.go      — mic capture via arecord
    ├── config/config.go       — load/save ~/.config/gotalk-dictation/config.json
    ├── hotkey/manager.go      — global X11 key grab (toggle + push-to-talk)
    ├── speech/
    │   ├── recognizer.go      — VAD + free/cloud API
    │   └── flac.go            — pure Go FLAC encoder
    ├── typing/typer.go        — xdotool/clipboard text insertion, punctuation, undo
    └── ui/
        ├── tray.go            — Fyne system tray + menu
        ├── settings.go        — settings window
        └── popup.go           — X11 animated overlay with transcript preview
```

## Roadmap

- **Segmented dictation** — send audio to the API on natural pauses so text appears clause-by-clause while speaking (works with free API)
- **Streaming dictation** — real-time interim results typed as you speak, corrected on final result (Google Cloud Speech API only)

## License

MIT — see LICENSE file.

---

### 👤 Ali Julaee Rad

[![GitHub followers](https://img.shields.io/github/followers/alijeyrad?label=Follow&style=social)](https://github.com/alijeyrad)

- **GitHub**: [alijeyrad](https://github.com/alijeyrad)
- **LinkedIn**: [in/ali-julaee-rad](https://www.linkedin.com/in/ali-julaee-rad/)
- **Email**: [alijrad.dev@gmail.com](mailto:alijrad.dev@gmail.com)
