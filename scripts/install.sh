#!/bin/sh
set -eu

# Crux installer script.
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/roygabriel/crux/main/scripts/install.sh | sh
#
# Environment variables:
#   CRUX_VERSION   - Version to install (default: latest)
#   INSTALL_DIR    - Installation directory (default: /usr/local/bin)

REPO="roygabriel/crux"
BINARY="crux"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

info() {
    printf '  \033[34m>\033[0m %s\n' "$@"
}

error() {
    printf '  \033[31mError\033[0m: %s\n' "$@" >&2
    exit 1
}

detect_os() {
    case "$(uname -s)" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "darwin" ;;
        *)       error "Unsupported operating system: $(uname -s)" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)  echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *)             error "Unsupported architecture: $(uname -m)" ;;
    esac
}

detect_downloader() {
    if command -v curl >/dev/null 2>&1; then
        echo "curl"
    elif command -v wget >/dev/null 2>&1; then
        echo "wget"
    else
        error "Neither curl nor wget found. Install one and retry."
    fi
}

download() {
    url="$1"
    output="$2"
    downloader="$(detect_downloader)"

    case "$downloader" in
        curl) curl -fsSL -o "$output" "$url" ;;
        wget) wget -qO "$output" "$url" ;;
    esac
}

get_latest_version() {
    tmpfile="$(mktemp)"
    download "https://api.github.com/repos/${REPO}/releases/latest" "$tmpfile" || \
        error "Failed to fetch latest release. You may be rate-limited. Set CRUX_VERSION explicitly."

    version="$(grep '"tag_name"' "$tmpfile" | head -1 | sed 's/.*"tag_name": *"//;s/".*//')"
    rm -f "$tmpfile"

    if [ -z "$version" ]; then
        error "Could not determine latest version. Set CRUX_VERSION explicitly."
    fi
    echo "$version"
}

main() {
    os="$(detect_os)"
    arch="$(detect_arch)"

    if [ -n "${CRUX_VERSION:-}" ]; then
        version="$CRUX_VERSION"
    else
        info "Fetching latest version..."
        version="$(get_latest_version)"
    fi

    info "Installing ${BINARY} ${version} (${os}/${arch})"

    tarball="${BINARY}_${version#v}_${os}_${arch}.tar.gz"
    url="https://github.com/${REPO}/releases/download/${version}/${tarball}"
    checksum_url="https://github.com/${REPO}/releases/download/${version}/checksums.txt"

    tmpdir="$(mktemp -d)"
    trap 'rm -rf "$tmpdir"' EXIT

    info "Downloading ${url}..."
    download "$url" "${tmpdir}/${tarball}" || \
        error "Download failed. Check that version ${version} exists and has a release for ${os}/${arch}."

    # Verify checksum if available.
    if download "$checksum_url" "${tmpdir}/checksums.txt" 2>/dev/null; then
        info "Verifying checksum..."
        expected="$(grep "${tarball}" "${tmpdir}/checksums.txt" | awk '{print $1}')"
        if [ -n "$expected" ]; then
            if command -v sha256sum >/dev/null 2>&1; then
                actual="$(sha256sum "${tmpdir}/${tarball}" | awk '{print $1}')"
            elif command -v shasum >/dev/null 2>&1; then
                actual="$(shasum -a 256 "${tmpdir}/${tarball}" | awk '{print $1}')"
            else
                info "No sha256sum or shasum found, skipping checksum verification."
                actual="$expected"
            fi

            if [ "$actual" != "$expected" ]; then
                error "Checksum mismatch! Expected ${expected}, got ${actual}."
            fi
            info "Checksum verified."
        fi
    fi

    info "Extracting..."
    tar -xzf "${tmpdir}/${tarball}" -C "${tmpdir}"

    info "Installing to ${INSTALL_DIR}/${BINARY}..."
    if [ -w "$INSTALL_DIR" ]; then
        install -m 755 "${tmpdir}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    else
        info "Elevated permissions required to install to ${INSTALL_DIR}."
        sudo install -m 755 "${tmpdir}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    fi

    info "Installed ${BINARY} ${version} to ${INSTALL_DIR}/${BINARY}"
    echo ""
    info "Run '${BINARY} --version' to verify."
    info "Run '${BINARY} completion bash' for shell completions."
    info "Run '${BINARY} init' to initialize a new project."
}

main
