#!/bin/sh
# Armstrong installer for Linux and macOS.
#
#   curl -fsSL https://raw.githubusercontent.com/ark4660/armstrong/main/scripts/install.sh | sh
#
# Installs the latest launcher to ~/.local/bin/armstrong (no root needed, so
# the launcher can update itself), adds that folder to your PATH, and on Linux
# adds Armstrong to your applications menu. Run again to reinstall; pass
# --uninstall (… | sh -s -- --uninstall) to remove it.
set -eu

REPO="ark4660/armstrong"
RELEASES="${ARMSTRONG_RELEASES_URL:-https://github.com/$REPO/releases/latest/download}"
ASSETS="${ARMSTRONG_ASSETS_URL:-https://raw.githubusercontent.com/$REPO/main/assets}"
BIN_DIR="${ARMSTRONG_INSTALL_DIR:-$HOME/.local/bin}"
APPS_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/applications"
ICON_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/icons/hicolor/256x256/apps"
MARKER="# Added by the Armstrong installer"

say() { printf '%s\n' "$*"; }
fail() { printf 'armstrong install: %s\n' "$*" >&2; exit 1; }

fetch() { # fetch URL FILE
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$1" -o "$2"
	elif command -v wget >/dev/null 2>&1; then
		wget -q "$1" -O "$2"
	else
		fail "needs curl or wget"
	fi
}

sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d' ' -f1
	else
		shasum -a 256 "$1" | cut -d' ' -f1
	fi
}

uninstall() {
	rm -f "$BIN_DIR/armstrong" "$APPS_DIR/armstrong.desktop" "$ICON_DIR/armstrong.png"
	command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database -q "$APPS_DIR" 2>/dev/null || true
	say "Armstrong is uninstalled."
	say "Your instances and settings are still in ~/.local/share/armstrong (delete that folder to remove them),"
	say "and the PATH line in your shell profile (marked \"$MARKER\") can be removed by hand."
	exit 0
}

if [ "${1:-}" = "--uninstall" ]; then
	uninstall
fi

case "$(uname -s)" in
	Linux) os=linux ;;
	Darwin) os=darwin ;;
	*) fail "unsupported system $(uname -s); on Windows use install.ps1" ;;
esac
case "$(uname -m)" in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	*) fail "unsupported CPU $(uname -m)" ;;
esac
name="armstrong-$os-$arch"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

say "Downloading Armstrong for $os/$arch..."
fetch "$RELEASES/release.json" "$tmp/release.json" || fail "can't reach the release index"
version="$(sed -n 's/^  "version": "\(.*\)",$/\1/p' "$tmp/release.json")"
# The launcher entry's sha256 is the third line after its name in release.json.
want="$(grep -A3 "\"name\": \"$name\"" "$tmp/release.json" | sed -n 's/.*"sha256": "\([0-9a-f]*\)".*/\1/p')"
[ -n "$want" ] || fail "the latest release has no build for $os/$arch"
fetch "$RELEASES/$name" "$tmp/armstrong" || fail "download failed"
got="$(sha256 "$tmp/armstrong")"
[ "$got" = "$want" ] || fail "checksum mismatch (got $got, want $want); not installing"

mkdir -p "$BIN_DIR"
chmod 755 "$tmp/armstrong"
mv -f "$tmp/armstrong" "$BIN_DIR/armstrong"
if [ "$os" = darwin ]; then
	xattr -d com.apple.quarantine "$BIN_DIR/armstrong" 2>/dev/null || true
fi
say "Installed Armstrong $version to $BIN_DIR/armstrong"

# Make "armstrong" a command: put the folder on PATH in the shell's profile.
on_path=false
case ":$PATH:" in *":$BIN_DIR:"*) on_path=true ;; esac
if [ "$on_path" = false ]; then
	shell_name="$(basename "${SHELL:-sh}")"
	case "$shell_name" in
		bash) profile="$HOME/.bashrc" ;;
		zsh) profile="${ZDOTDIR:-$HOME}/.zshrc" ;;
		fish) profile="$HOME/.config/fish/conf.d/armstrong.fish" ;;
		*) profile="$HOME/.profile" ;;
	esac
	if ! grep -qs "$MARKER" "$profile"; then
		mkdir -p "$(dirname "$profile")"
		if [ "$shell_name" = fish ]; then
			printf '\n%s\nfish_add_path "%s"\n' "$MARKER" "$BIN_DIR" >>"$profile"
		else
			printf '\n%s\nexport PATH="%s:$PATH"\n' "$MARKER" "$BIN_DIR" >>"$profile"
		fi
		say "Added $BIN_DIR to your PATH in $profile"
	fi
fi

# On Linux, list it in the applications menu (it opens in your terminal).
if [ "$os" = linux ]; then
	mkdir -p "$APPS_DIR" "$ICON_DIR"
	if fetch "$ASSETS/armstrong.png" "$tmp/armstrong.png"; then
		mv -f "$tmp/armstrong.png" "$ICON_DIR/armstrong.png"
	else
		say "(couldn't download the icon; the menu entry will use a default one)"
	fi
	cat >"$APPS_DIR/armstrong.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=Armstrong
GenericName=Minecraft Launcher
Comment=Terminal Minecraft launcher and performance client
Exec=$BIN_DIR/armstrong
Icon=armstrong
Terminal=true
Categories=Game;
Keywords=minecraft;launcher;fabric;
EOF
	command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database -q "$APPS_DIR" 2>/dev/null || true
	command -v gtk-update-icon-cache >/dev/null 2>&1 && gtk-update-icon-cache -q "${ICON_DIR%/256x256/apps}" 2>/dev/null || true
	say "Added Armstrong to your applications menu"
fi

say ""
if [ "$on_path" = true ]; then
	say "Done. Run: armstrong"
else
	say "Done. Open a new terminal and run: armstrong"
fi
