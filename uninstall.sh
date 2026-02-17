#!/usr/bin/env bash
# GoTalk Dictation uninstaller
# Removes everything installed by install.sh or make install.
#
# Usage:
#   ./uninstall.sh
#
# What it removes (system-wide, requires sudo):
#   /usr/local/bin/gotalk-dictation
#   /usr/share/applications/com.alijeyrad.GoTalkDictation.desktop
#   /usr/share/metainfo/com.alijeyrad.GoTalkDictation.metainfo.xml
#   /usr/share/icons/hicolor/128x128/apps/com.alijeyrad.GoTalkDictation.png
#
# What it removes (per-user, no sudo):
#   ~/.local/share/applications/com.alijeyrad.GoTalkDictation.desktop
#   ~/.config/autostart/gotalk-dictation.desktop
#
# Optionally removes (prompted):
#   ~/.config/gotalk-dictation/   (config file with all your settings)
set -euo pipefail

BINARY="gotalk-dictation"

RED='\033[0;31m'; GREEN='\033[0;32m'; BLUE='\033[0;34m'; YELLOW='\033[1;33m'; NC='\033[0m'
info() { echo -e "${BLUE}[gotalk]${NC} $*"; }
ok()   { echo -e "${GREEN}[gotalk]${NC} $*"; }
warn() { echo -e "${YELLOW}[gotalk]${NC} $*"; }

echo ""
echo "  GoTalk Dictation — Uninstaller"
echo ""

# ── stop any running instance ───────────────────────────────────────────────────

if pgrep -x "$BINARY" >/dev/null 2>&1; then
    info "Stopping running instance…"
    pkill -x "$BINARY" 2>/dev/null || true
    sleep 0.5
    ok "Process stopped."
fi

# ── remove system-wide files ────────────────────────────────────────────────────

info "Removing system files (sudo required)…"

sudo rm -f /usr/local/bin/$BINARY
sudo rm -f /usr/share/applications/com.alijeyrad.GoTalkDictation.desktop
sudo rm -f /usr/share/metainfo/com.alijeyrad.GoTalkDictation.metainfo.xml
sudo rm -f /usr/share/icons/hicolor/128x128/apps/com.alijeyrad.GoTalkDictation.png

sudo gtk-update-icon-cache -f -t /usr/share/icons/hicolor 2>/dev/null || true
sudo update-desktop-database /usr/share/applications 2>/dev/null || true

ok "System files removed."

# ── remove per-user files ───────────────────────────────────────────────────────

removed_user=false

if [[ -f "$HOME/.local/share/applications/com.alijeyrad.GoTalkDictation.desktop" ]]; then
    rm -f "$HOME/.local/share/applications/com.alijeyrad.GoTalkDictation.desktop"
    update-desktop-database "$HOME/.local/share/applications" 2>/dev/null || true
    ok "Removed user desktop entry."
    removed_user=true
fi

if [[ -f "$HOME/.config/autostart/$BINARY.desktop" ]]; then
    rm -f "$HOME/.config/autostart/$BINARY.desktop"
    ok "Removed autostart entry."
    removed_user=true
fi

$removed_user || info "No per-user desktop/autostart files found."

# ── optionally remove config ────────────────────────────────────────────────────

CONFIG_DIR="$HOME/.config/gotalk-dictation"

if [[ -d "$CONFIG_DIR" ]]; then
    echo ""
    read -r -p "  Remove your settings and config (~/.config/gotalk-dictation/)? [y/N] " ans
    if [[ "${ans,,}" == "y" ]]; then
        rm -rf "$CONFIG_DIR"
        ok "Config directory removed."
    else
        warn "Config kept at $CONFIG_DIR"
    fi
fi

echo ""
ok "GoTalk Dictation has been uninstalled."
echo ""
