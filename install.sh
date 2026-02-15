#!/usr/bin/env bash
# install.sh — GoTalk Dictation release installer
# Installs the pre-built binary, desktop entry, and runtime dependencies.
# Run as a normal user; sudo is invoked only where needed.
set -euo pipefail

BINARY=gotalk-dictation
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

RED='\033[0;31m'; GREEN='\033[0;32m'; BLUE='\033[0;34m'; YELLOW='\033[1;33m'; NC='\033[0m'
info()  { echo -e "${BLUE}[INFO]${NC} $*"; }
ok()    { echo -e "${GREEN}[OK]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
die()   { echo -e "${RED}[ERR]${NC} $*" >&2; exit 1; }

# ── runtime dependencies (no build deps needed — binary is pre-compiled) ──────

install_runtime_deps() {
    info "Installing runtime dependencies (alsa-utils, xdotool, xclip)…"
    if command -v apt-get >/dev/null 2>&1; then
        sudo apt-get install -y alsa-utils xdotool xclip
    elif command -v dnf >/dev/null 2>&1; then
        sudo dnf install -y alsa-utils xdotool xclip
    elif command -v pacman >/dev/null 2>&1; then
        sudo pacman -S --needed --noconfirm alsa-utils xdotool xclip
    else
        warn "Unknown package manager. Please install manually: alsa-utils xdotool xclip"
    fi
}

# ── install binary + desktop files ────────────────────────────────────────────

install_files() {
    info "Installing $BINARY to /usr/local/bin/…"
    sudo install -m 755 "$SCRIPT_DIR/$BINARY" /usr/local/bin/$BINARY

    info "Installing desktop entry…"
    sudo install -Dm 644 \
        "$SCRIPT_DIR/packaging/com.alijeyrad.GoTalkDictation.desktop" \
        /usr/share/applications/com.alijeyrad.GoTalkDictation.desktop

    if [[ -f "$SCRIPT_DIR/packaging/com.alijeyrad.GoTalkDictation.metainfo.xml" ]]; then
        sudo install -Dm 644 \
            "$SCRIPT_DIR/packaging/com.alijeyrad.GoTalkDictation.metainfo.xml" \
            /usr/share/metainfo/com.alijeyrad.GoTalkDictation.metainfo.xml
    fi
}

# ── optional autostart ─────────────────────────────────────────────────────────

setup_autostart() {
    local autostart_dir="$HOME/.config/autostart"
    mkdir -p "$autostart_dir"
    cat > "$autostart_dir/$BINARY.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=GoTalk Dictation
Exec=/usr/local/bin/$BINARY
Icon=audio-input-microphone
Comment=System-wide speech-to-text dictation
Categories=Accessibility;Utility;
X-GNOME-Autostart-enabled=true
EOF
    ok "Autostart entry created: $autostart_dir/$BINARY.desktop"
}

# ── cleanup ────────────────────────────────────────────────────────────────────

cleanup() {
    info "Removing installation files from $SCRIPT_DIR…"
    rm -rf "$SCRIPT_DIR"
    ok "Cleaned up."
}

# ── main ───────────────────────────────────────────────────────────────────────

echo ""
echo "  GoTalk Dictation — Installer"
echo "  ──────────────────────────────"
echo ""

[[ -f "$SCRIPT_DIR/$BINARY" ]] || die "Binary '$BINARY' not found in $SCRIPT_DIR"

install_runtime_deps
install_files

ok ""
ok "  Installed! You can now run: $BINARY"
ok "  Or launch it from your application menu."
ok ""

read -r -p "Add GoTalk to autostart (runs at login)? [y/N] " ans
[[ "${ans,,}" == "y" ]] && setup_autostart

read -r -p "Remove these installation files? [Y/n] " ans
ans="${ans:-Y}"
[[ "${ans,,}" == "y" ]] && cleanup || ok "Files kept in $SCRIPT_DIR."

echo ""
