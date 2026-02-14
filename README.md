# GoTalk Dictation

A fast, native Linux speech-to-text application built with Go and Fyne. Convert your speech to text anywhere on your system with a simple keyboard shortcut.

![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Go Version](https://img.shields.io/badge/go-1.25.7-blue.svg)

## Features

- 🎤 **System-wide dictation** - Works in any application
- ⚡ **Fast & Native** - Written in pure Go, no Python dependencies
- ⌨️ **Global hotkey** - Default: Alt+D (customizable)
- 🌍 **Multi-language support** - English, Spanish, and more
- 🔔 **Desktop notifications** - Visual feedback during dictation but minimal
- 🎯 **Punctuation commands** - Say "period", "comma", "question mark"
- 🎨 **System tray integration** - Quick access from anywhere
- 🔒 **Privacy-focused** - Uses Google Cloud Speech API (requires API key)
- 🐧 **Linux native** - Built specifically for Linux desktop environments

## Architecture

```
gotalk-dictation/
├── main.go                     # Application entry point
├── internal/
│   ├── speech/
│   │   └── recognizer.go      # Google Cloud Speech integration
│   ├── audio/
│   │   └── recorder.go        # ALSA/PulseAudio audio capture
│   ├── hotkey/
│   │   └── manager.go         # Global hotkey registration
│   ├── ui/
│   │   ├── tray.go            # System tray icon and menu
│   │   ├── settings.go        # Settings window
│   │   └── history.go         # Dictation history viewer
│   ├── config/
│   │   └── config.go          # Configuration management
│   └── typing/
│       └── typer.go           # X11 keyboard simulation (xdotool)
├── assets/
│   ├── icon.png               # Application icon
│   └── tray-icons/            # System tray icons (various states)
├── go.mod
├── go.sum
└── README.md
```

## Prerequisites

### System Dependencies

```bash
# Fedora/RHEL
sudo dnf install -y libx11-devel libxcursor-devel libxrandr-devel \
  libxinerama-devel libxi-devel libgl-devel alsa-lib-devel \
  xdotool libnotify

# Ubuntu/Debian
sudo apt install -y libx11-dev libxcursor-dev libxrandr-dev \
  libxinerama-dev libxi-dev libgl1-mesa-dev libasound2-dev \
  xdotool libnotify-bin

# Arch
sudo pacman -S libx11 libxcursor libxrandr libxinerama libxi \
  mesa alsa-lib xdotool libnotify
```

### Google Cloud Speech API

1. Create a Google Cloud project: https://console.cloud.google.com
2. Enable Speech-to-Text API
3. Create a service account and download JSON key
4. Set environment variable:

```bash
   export GOOGLE_APPLICATION_CREDENTIALS="/path/to/service-account-key.json"
```

**OR** use the free tier without authentication (limited to 60 minutes/month):

- The app will use Google's free public API endpoint

## Installation

### From Source

```bash
# Clone repository
git clone https://github.com/Alijeyrad/gotalk-dictation.git
cd gotalk-dictation

# Install dependencies
go mod download

# Build
go build -o gotalk-dictation

# Install (optional)
sudo install -m 755 gotalk-dictation /usr/local/bin/
```

### Run

```bash
# Run directly
./gotalk-dictation

# Or if installed
gotalk-dictation
```

## Usage

### Basic Dictation

1. Press **Alt+D** (or your configured hotkey)
2. Wait for the "🎤 Listening..." notification
3. Speak clearly into your microphone
4. Text will be typed automatically at your cursor position

### Punctuation Commands

Say these words to insert punctuation:

- "period" → `.`
- "comma" → `,`
- "question mark" → `?`
- "exclamation mark" → `!`
- "new line" → `↵`
- "new paragraph" → `↵↵`
- "colon" → `:`
- "semicolon" → `;`

### System Tray

Right-click the system tray icon for:

- Start/Stop Dictation
- Settings
- Quit

## Configuration

Configuration file location: `~/.config/gotalk-dictation/config.json`

```json
{
  "hotkey": "Alt+D",
  "language": "en-US",
  "timeout": 30,
  "phrase_time_limit": 60,
  "enable_punctuation": true,
  "enable_notifications": true,
}
```

## Development

### Project Structure

**main.go**

- Application initialization
- Window management
- Main event loop

**internal/speech/recognizer.go**

- Google Cloud Speech API integration
- Audio streaming
- Real-time transcription
- Language detection

**internal/audio/recorder.go**

- ALSA/PulseAudio audio capture
- Microphone input handling
- Audio buffer management
- Format conversion (PCM → WAV)

**internal/hotkey/manager.go**

- Global hotkey registration using robotgo
- X11 event monitoring
- Hotkey conflict detection

**internal/ui/tray.go**

- System tray icon with fyne/systray
- Context menu
- Status indicators (listening, idle, error)

**internal/ui/settings.go**

- Fyne settings window
- Language selection
- Hotkey customization
- Audio device selection

**internal/ui/history.go**

- Dictation history viewer
- Search and filter
- Copy to clipboard
- Delete history items

**internal/config/config.go**

- JSON configuration file management
- Default settings
- Config validation
- Hot reload support

**internal/typing/typer.go**

- X11 keyboard simulation via xdotool
- Text insertion at cursor
- Special character handling
- Wayland compatibility (future)

### Key Technical Details

**Audio Recording**

```go
// Use ALSA to capture microphone input
// Format: 16-bit PCM, 16000 Hz mono (required by Google Speech API)
// Buffer size: 4096 samples
// Use Go channels for streaming audio data
```

**Speech Recognition**

```go
// Use cloud.google.com/go/speech/apiv1
// StreamingRecognize for real-time transcription
// RecognitionConfig:
//   - Encoding: LINEAR16
//   - SampleRateHertz: 16000
//   - LanguageCode: from config (default: en-US)
//   - EnableAutomaticPunctuation: false (we handle it manually)
```

**Global Hotkey**

```go
// Use robotgo.EventHook for X11 keyboard events
// Register hotkey combinations (e.g., Alt+D)
// Handle conflicts gracefully
// Wayland support: Use D-Bus portal API (future)
```

**Text Insertion**

```go
// Use xdotool for typing text at cursor position
// Command: xdotool type --clearmodifiers -- "text"
// Handle special characters and Unicode
// Preserve clipboard contents
```

### Building

```bash
# Development build
go build -o gotalk-dictation

# Production build (optimized)
go build -ldflags="-s -w" -o gotalk-dictation

# Cross-compile for different architectures
GOOS=linux GOARCH=amd64 go build -o gotalk-dictation-amd64
GOOS=linux GOARCH=arm64 go build -o gotalk-dictation-arm64
```

### Testing

```bash
# Run tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific test
go test -v ./internal/speech/

# Benchmark
go test -bench=. ./internal/audio/
```

## Packaging

### Flatpak

```bash
# Build Flatpak
flatpak-builder build com.alijeyrad.gotalk-dictation.yml

# Install locally
flatpak-builder --user --install build com.alijeyrad.gotalk-dictation.yml

# Run
flatpak run com.alijeyrad.gotalk-dictation
```

### AppImage

```bash
# Use appimagetool
wget https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-x86_64.AppImage
chmod +x appimagetool-x86_64.AppImage

# Create AppDir
./scripts/create-appimage.sh

# Build AppImage
./appimagetool-x86_64.AppImage AppDir gotalk-dictation.AppImage
```

## Roadmap

- [ ] Core dictation functionality (v0.1.0)

  - [X] Google Cloud Speech integration
  - [X] Audio recording
  - [X] Global hotkey
  - [X] Text insertion
  - [ ] Basic UI
- [ ] Enhanced features (v0.2.0)

  - [ ] Punctuation commands
  - [ ] Multi-language support
  - [ ] Settings window
  - [ ] System tray
- [ ] Advanced features (v0.3.0)

  - [ ] Dictation history
  - [ ] Custom commands
  - [ ] Offline mode (Vosk integration)
  - [ ] Wayland support
- [ ] Polish & Distribution (v1.0.0)

  - [ ] Flatpak package
  - [ ] AppImage package
  - [ ] Publish to Flathub
  - [ ] Documentation website

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

MIT License - see LICENSE file for details

## Credits

- Built with [Fyne](https://fyne.io/) - Cross-platform UI toolkit
- Speech recognition powered by [Google Cloud Speech-to-Text](https://cloud.google.com/speech-to-text)
- Global hotkeys via [RobotGo](https://github.com/go-vgo/robotgo)
- Icon design: [Your name/attribution]

## Support

- 🐛 Report bugs: [GitHub Issues](https://github.com/Alijeyrad/gotalk-dictation/issues)
- 💬 Discussions: [GitHub Discussions](https://github.com/Alijeyrad/gotalk-dictation/discussions)
- 📧 Email: alijrad.dev@gmail.com

---

**Made with ❤️ by Ali Julaee Rad**
