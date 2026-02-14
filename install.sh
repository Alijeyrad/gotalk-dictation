#!/usr/bin/env bash
# GoTalk Dictation — dependency installer and build script
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info()    { echo -e "${BLUE}[INFO]${NC} $*"; }
success() { echo -e "${GREEN}[OK]${NC} $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC} $*"; }
die()     { echo -e "${RED}[ERR]${NC} $*" >&2; exit 1; }

# ---- detect distro ----------------------------------------------------------

detect_distro() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        echo "${ID}"
    else
        die "Cannot detect Linux distribution."
    fi
}

DISTRO=$(detect_distro)

# ---- system dependencies ----------------------------------------------------

install_deps_fedora() {
    info "Installing system dependencies (Fedora/RHEL)..."
    sudo dnf install -y \
        libX11-devel \
        libXcursor-devel \
        libXrandr-devel \
        libXinerama-devel \
        libXi-devel \
        libXxf86vm-devel \
        mesa-libGL-devel \
        alsa-utils \
        xdotool \
        xclip
}

install_deps_ubuntu() {
    info "Installing system dependencies (Ubuntu/Debian)..."
    sudo apt-get update -q
    sudo apt-get install -y \
        libx11-dev \
        libxcursor-dev \
        libxrandr-dev \
        libxinerama-dev \
        libxi-dev \
        libxxf86vm-dev \
        libgl1-mesa-dev \
        alsa-utils \
        xdotool \
        xclip
}

install_deps_arch() {
    info "Installing system dependencies (Arch)..."
    sudo pacman -S --needed \
        libx11 \
        libxcursor \
        libxrandr \
        libxinerama \
        libxi \
        libxxf86vm \
        mesa \
        alsa-utils \
        xdotool \
        xclip
}

case "$DISTRO" in
    fedora|rhel|centos|rocky|almalinux)
        install_deps_fedora ;;
    ubuntu|debian|linuxmint|pop)
        install_deps_ubuntu ;;
    arch|manjaro|endeavouros|garuda)
        install_deps_arch ;;
    *)
        warn "Unknown distro '$DISTRO'. Checking for required tools manually..."
        MISSING=()
        for tool in arecord xdotool xclip; do
            command -v "$tool" &>/dev/null || MISSING+=("$tool")
        done
        if [ ${#MISSING[@]} -gt 0 ]; then
            die "Missing tools: ${MISSING[*]}. Install them with your package manager and re-run."
        fi
        ;;
esac

success "System dependencies installed."

# ---- Go toolchain -----------------------------------------------------------

if ! command -v go &>/dev/null; then
    die "Go is not installed. Install from https://go.dev/dl/ and re-run."
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
info "Go version: $GO_VERSION"

# ---- build ------------------------------------------------------------------

info "Downloading Go module dependencies..."
go mod download

info "Building gotalk-dictation..."
go build -ldflags="-s -w" -o gotalk-dictation .

success "Built: $(pwd)/gotalk-dictation"

# ---- optional install -------------------------------------------------------

read -r -p "Install to /usr/local/bin? [y/N] " REPLY
if [[ "${REPLY,,}" == "y" ]]; then
    sudo install -m 755 gotalk-dictation /usr/local/bin/gotalk-dictation
    success "Installed to /usr/local/bin/gotalk-dictation"

    # Install a system desktop file so KDE/PipeWire can identify the app and
    # remember the microphone permission grant across sessions.
    sudo tee /usr/share/applications/gotalk-dictation.desktop >/dev/null <<'EOF'
[Desktop Entry]
Type=Application
Name=GoTalk Dictation
Comment=System-wide speech-to-text dictation
Exec=gotalk-dictation
Icon=audio-input-microphone
Categories=Accessibility;Utility;
NoDisplay=true
EOF
    success "Desktop entry installed to /usr/share/applications/gotalk-dictation.desktop"
fi

# ---- hotkey setup reminder --------------------------------------------------

echo ""
echo -e "${YELLOW}━━━ Hotkey Setup ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo "  GoTalk uses Alt+D as its global hotkey."
echo ""
echo "  If Alt+D is already bound in your DE (e.g. an old dictation script):"
echo "    GNOME:  Settings → Keyboard → Keyboard Shortcuts → find Alt+D → remove"
echo "    KDE:    System Settings → Shortcuts → find Alt+D → remove"
echo "    XFCE:   Settings → Keyboard → Application Shortcuts → find Alt+D → remove"
echo ""
echo "  To use a different hotkey, edit:"
echo "    ~/.config/gotalk-dictation/config.json  → change \"hotkey\": \"Alt-d\""
echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# ---- autostart (optional) ---------------------------------------------------

AUTOSTART_DIR="${HOME}/.config/autostart"
DESKTOP_FILE="${AUTOSTART_DIR}/gotalk-dictation.desktop"
BINARY_PATH="$(command -v gotalk-dictation 2>/dev/null || echo "$(pwd)/gotalk-dictation")"

read -r -p "Add to autostart (runs at login)? [y/N] " REPLY
if [[ "${REPLY,,}" == "y" ]]; then
    mkdir -p "$AUTOSTART_DIR"
    cat > "$DESKTOP_FILE" <<EOF
[Desktop Entry]
Type=Application
Name=GoTalk Dictation
Exec=${BINARY_PATH}
Icon=audio-input-microphone
Comment=System-wide speech-to-text dictation
Categories=Accessibility;Utility;
X-GNOME-Autostart-enabled=true
EOF
    success "Autostart entry created: $DESKTOP_FILE"
fi

echo ""
success "Done! Run with:  ./gotalk-dictation"
