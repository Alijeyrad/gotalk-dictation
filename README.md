# GoTalk Dictation

A fast, native Linux speech-to-text app. Press a hotkey anywhere, speak, and the transcribed text is typed at your cursor.

![License](https://img.shields.io/badge/license-MIT-blue.svg)

## Features

- **System-wide dictation** — works in any application
- **Global hotkey** — default `Alt+D`, fully rebindable live from Settings
- **Visual indicator** — small X11 overlay shows listening / processing / done / error states
- **Voice Activity Detection** — auto-stops after silence; configurable sensitivity and silence duration
- **Punctuation commands** — say "period", "comma", "question mark", etc.
- **Multi-language** — English (US), Spanish, French, German, Persian (Farsi); easily extended
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
`xdotool` types short transcripts directly; `xclip` is used for paste-based insertion of longer text (faster for long dictations).

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
go build -ldflags="-s -w" -o gotalk-dictation
sudo install -m 755 gotalk-dictation /usr/local/bin/
```

Or use the provided install script:

```bash
./install.sh
```

## Usage

1. Run `gotalk-dictation` — it appears in the system tray.
2. Press **Alt+D** (or your configured hotkey).
3. Speak. The floating indicator shows the current state.
4. Text is typed at the cursor when you stop speaking.

Press the hotkey again while listening to cancel.

### Punctuation commands

| Say               | Gets typed |
| ----------------- | ---------- |
| period            | `.`      |
| comma             | `,`      |
| question mark     | `?`      |
| exclamation mark  | `!`      |
| colon             | `:`      |
| semicolon         | `;`      |
| new line          | `↵`     |
| new paragraph     | `↵↵`   |
| open parenthesis  | `(`      |
| close parenthesis | `)`      |
| dash / hyphen     | `-`      |
| ellipsis          | `...`    |

## Settings

Open **Settings** from the tray icon. All changes apply immediately — no restart needed.

| Setting                     | Description                                               |
| --------------------------- | --------------------------------------------------------- |
| Language                    | Speech recognition language                               |
| Custom API key              | Override the built-in Chromium key for the free API       |
| Use Google Cloud Speech API | Switch to the Cloud API (requires credentials)            |
| Silence end                 | How long a pause ends the phrase (~62 ms per chunk)       |
| Sensitivity                 | RMS threshold multiplier — lower picks up quieter voices |
| Hotkey                      | Click the button and press any modifier+key combination   |
| Max duration                | Hard timeout for a single dictation session               |
| Add punctuation             | Enable spoken punctuation commands                        |

### Google Cloud Speech API (optional)

For higher accuracy, enable the Cloud API and set credentials:

```bash
export GOOGLE_APPLICATION_CREDENTIALS="/path/to/service-account-key.json"
# or run: gcloud auth application-default login
```

Without credentials the free public endpoint is used — no account needed.

## Configuration file

`~/.config/gotalk-dictation/config.json` is written automatically by the Settings window.

```json
{
  "hotkey": "Alt-d",
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
├── main.go
└── internal/
    ├── audio/recorder.go      — mic capture via arecord
    ├── config/config.go       — load/save ~/.config/gotalk-dictation/config.json
    ├── hotkey/manager.go      — global X11 key grab
    ├── speech/
    │   ├── recognizer.go      — VAD + free/cloud API
    │   └── flac.go            — pure Go FLAC encoder
    ├── typing/typer.go        — xdotool text insertion + punctuation
    └── ui/
        ├── tray.go            — Fyne system tray + menu
        ├── settings.go        — settings window
        └── popup.go           — X11 animated overlay
```

## License

MIT — see LICENSE file.

---

### 👤 Ali Julaee Rad

[![GitHub followers](https://img.shields.io/github/followers/alijeyrad?label=Follow&style=social)](https://github.com/alijeyrad)

- **GitHub**: [alijeyrad](https://github.com/alijeyrad)
- **LinkedIn**: [in/ali-julaee-rad](https://www.linkedin.com/in/ali-julaee-rad/)
- **Email**: [alijrad.dev@gmail.com](mailto:alijrad.dev@gmail.com)