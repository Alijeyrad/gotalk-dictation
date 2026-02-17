# GoTalk Dictation

Inspired by macOS dictation — a global hotkey that lets you speak and have the words typed wherever your cursor is. I built it to scratch my own itch: Linux didn't have anything that just worked the same way. I use it every day, mostly for typing long prompts and messages without touching the keyboard.

[![CI](https://github.com/Alijeyrad/gotalk-dictation/actions/workflows/ci.yml/badge.svg)](https://github.com/Alijeyrad/gotalk-dictation/actions/workflows/ci.yml)
[![Latest release](https://img.shields.io/github/v/release/Alijeyrad/gotalk-dictation)](https://github.com/Alijeyrad/gotalk-dictation/releases/latest)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## How it works

Press **Alt+D** anywhere, speak, and the transcribed text is typed at your cursor. It stops automatically when you go silent. Press the hotkey again to cancel, or **Alt+Z** to undo what was just typed.

There's also a push-to-talk mode: configure a separate hotkey in Settings, hold it while speaking, and release to submit.

Uses Google's speech recognition — free, no account needed. For higher accuracy, you can configure the Google Cloud Speech API with your own credentials.

## Installation

```bash
curl -fsSL https://github.com/Alijeyrad/gotalk-dictation/releases/latest/download/install.sh | bash
```

### Flatpak (recommended)

> Flathub submission in progress. See [`packaging/flatpak/`](packaging/flatpak/) for manual Flatpak build instructions.

### From a release

Download from the [Releases](https://github.com/Alijeyrad/gotalk-dictation/releases/latest) page and run `./install.sh`, or install manually:

```bash
sudo install -m755 gotalk-dictation /usr/local/bin/
sudo install -m644 com.alijeyrad.GoTalkDictation.desktop /usr/share/applications/
```

### From source

```bash
git clone https://github.com/Alijeyrad/gotalk-dictation.git
cd gotalk-dictation
make deps      # install system build deps (X11 headers, GL)
make install   # build and install to /usr/local/bin + .desktop + icon
make autostart # optional: start at login
```

## Uninstall

```bash
./uninstall.sh
```

The script stops any running instance, removes all system-wide and per-user files (binary, desktop entry, icon, autostart), and prompts before deleting your settings.

If you installed from source:

```bash
make uninstall
```

This does the same non-interactively — your config at `~/.config/gotalk-dictation/` is preserved. Remove it manually if you want a completely clean slate:

```bash
rm -rf ~/.config/gotalk-dictation/
```

## Settings

Open **Settings** from the tray icon. Everything takes effect immediately.

- **Language** — 25 languages and regional variants
- **Hotkey / Push-to-talk hotkey / Undo hotkey** — click the field and press any modifier+key
- **Silence end** — how long a pause ends the phrase
- **Sensitivity** — RMS threshold; lower picks up quieter voices
- **Add punctuation** — say "period", "comma", "new line", etc. to insert them
- **Max duration** — hard timeout per session
- **Google Cloud Speech API** — set `GOOGLE_APPLICATION_CREDENTIALS` or use `gcloud auth application-default login` for higher accuracy

## License

MIT — see [LICENSE](LICENSE).
