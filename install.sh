#!/usr/bin/env bash
# GoTalk Dictation installer
# Works two ways:
#   1. curl -fsSL https://github.com/Alijeyrad/gotalk-dictation/releases/latest/download/install.sh | bash
#   2. ./install.sh  (from an extracted release tarball)
set -euo pipefail

REPO="Alijeyrad/gotalk-dictation"
BINARY="gotalk-dictation"
tmpdir=""

RED='\033[0;31m'; GREEN='\033[0;32m'; BLUE='\033[0;34m'; YELLOW='\033[1;33m'; NC='\033[0m'
info() { echo -e "${BLUE}[gotalk]${NC} $*"; }
ok()   { echo -e "${GREEN}[gotalk]${NC} $*"; }
warn() { echo -e "${YELLOW}[gotalk]${NC} $*"; }
die()  { echo -e "${RED}[gotalk]${NC} $*" >&2; exit 1; }

# ── detect local vs remote mode ────────────────────────────────────────────────

SCRIPT_DIR=""
if [[ -n "${BASH_SOURCE[0]:-}" && "${BASH_SOURCE[0]}" != "bash" ]]; then
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fi

if [[ -n "$SCRIPT_DIR" && -f "$SCRIPT_DIR/$BINARY" ]]; then
    LOCAL=true
else
    LOCAL=false
fi

# ── download latest release ────────────────────────────────────────────────────

download_release() {
    command -v curl >/dev/null 2>&1 || die "curl is required"

    info "Fetching latest release…"
    local api="https://api.github.com/repos/${REPO}/releases/latest"
    local version
    version=$(curl -fsSL "$api" | grep '"tag_name"' | head -1 | cut -d'"' -f4 | sed 's/^v//')
    [[ -n "$version" ]] || die "Could not determine latest version"

    local arch
    arch=$(uname -m)
    [[ "$arch" == "x86_64" ]] || die "Only x86_64 is supported (got $arch)"

    local tarball="gotalk-dictation_${version}_linux_amd64.tar.gz"
    local url="https://github.com/${REPO}/releases/download/v${version}/${tarball}"

    info "Downloading v${version}…"
    tmpdir=$(mktemp -d)
    trap 'rm -rf "${tmpdir:-}"' EXIT

    curl -fsSL "$url" -o "$tmpdir/$tarball"
    tar -xzf "$tmpdir/$tarball" -C "$tmpdir"
    SCRIPT_DIR="$tmpdir"
}

# ── install files ──────────────────────────────────────────────────────────────

install_files() {
    info "Installing $BINARY…"
    sudo install -m755 "$SCRIPT_DIR/$BINARY" /usr/local/bin/$BINARY

    sudo install -Dm644 \
        "$SCRIPT_DIR/packaging/com.alijeyrad.GoTalkDictation.desktop" \
        /usr/share/applications/com.alijeyrad.GoTalkDictation.desktop

    if [[ -f "$SCRIPT_DIR/packaging/com.alijeyrad.GoTalkDictation.metainfo.xml" ]]; then
        sudo install -Dm644 \
            "$SCRIPT_DIR/packaging/com.alijeyrad.GoTalkDictation.metainfo.xml" \
            /usr/share/metainfo/com.alijeyrad.GoTalkDictation.metainfo.xml
    fi

    if [[ -f "$SCRIPT_DIR/internal/ui/assets/icon.png" ]]; then
        sudo mkdir -p /usr/share/icons/hicolor/128x128/apps
        sudo install -m644 \
            "$SCRIPT_DIR/internal/ui/assets/icon.png" \
            /usr/share/icons/hicolor/128x128/apps/com.alijeyrad.GoTalkDictation.png
        sudo gtk-update-icon-cache -f -t /usr/share/icons/hicolor 2>/dev/null || true
    fi
}

# ── autostart ──────────────────────────────────────────────────────────────────

setup_autostart() {
    mkdir -p "$HOME/.config/autostart"
    cat > "$HOME/.config/autostart/$BINARY.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=GoTalk Dictation
Exec=/usr/local/bin/$BINARY
Icon=com.alijeyrad.GoTalkDictation
Comment=System-wide speech-to-text dictation
Categories=Accessibility;Utility;
X-GNOME-Autostart-enabled=true
EOF
    ok "Autostart entry created."
}

# ── main ───────────────────────────────────────────────────────────────────────

echo ""
echo "  GoTalk Dictation — Installer"
echo ""

$LOCAL || download_release

install_files

echo ""
ok "Installed! Run: $BINARY"
echo ""

read -r -p "  Add GoTalk to autostart (run at login)? [y/N] " ans
[[ "${ans,,}" == "y" ]] && setup_autostart

echo ""
