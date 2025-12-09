#!/bin/sh
# cpx installer - C++ Project Generator
# Usage: sh -c "$(curl -fsSL https://raw.githubusercontent.com/ozacod/cpx/master/install.sh)"

set -e

REPO="ozacod/cpx"
BINARY_NAME="cpx"

# Determine install directory (prefer user-local, fallback to system with sudo)
get_install_dir() {
    # Check for user-specified directory
    if [ -n "$CPX_INSTALL_DIR" ]; then
        echo "$CPX_INSTALL_DIR"
        return
    fi
    
    # Prefer ~/.local/bin (no sudo needed)
    LOCAL_BIN="$HOME/.local/bin"
    if [ -d "$LOCAL_BIN" ] && [ -w "$LOCAL_BIN" ]; then
        echo "$LOCAL_BIN"
        return
    fi
    
    # Check if /usr/local/bin is writable
    if [ -w "/usr/local/bin" ]; then
        echo "/usr/local/bin"
        return
    fi
    
    # Default to ~/.local/bin (will be created)
    echo "$LOCAL_BIN"
}

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

print_banner() {
    printf "\n"
    printf "%b   ██████╗██████╗ ██╗  ██╗%b\n" "$CYAN" "$NC"
    printf "%b  ██╔════╝██╔══██╗╚██╗██╔╝%b\n" "$CYAN" "$NC"
    printf "%b  ██║     ██████╔╝ ╚███╔╝ %b\n" "$CYAN" "$NC"
    printf "%b  ██║     ██╔═══╝  ██╔██╗ %b\n" "$CYAN" "$NC"
    printf "%b  ╚██████╗██║     ██╔╝ ██╗%b\n" "$CYAN" "$NC"
    printf "%b   ╚═════╝╚═╝     ╚═╝  ╚═╝%b\n" "$CYAN" "$NC"
    printf "\n"
    printf "  %bC++ Project Generator - Cpx Your Code!%b\n" "$YELLOW" "$NC"
    printf "\n"
}

detect_os() {
    OS="$(uname -s)"
    case "$OS" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "darwin" ;;
        MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
        *)       echo "unknown" ;;
    esac
}

detect_arch() {
    ARCH="$(uname -m)"
    case "$ARCH" in
        x86_64|amd64) echo "amd64" ;;
        arm64|aarch64) echo "arm64" ;;
        *)            echo "unknown" ;;
    esac
}

check_dependencies() {
    if ! command -v curl > /dev/null 2>&1; then
        if ! command -v wget > /dev/null 2>&1; then
            printf "%bError: curl or wget is required%b\n" "$RED" "$NC"
            exit 1
        fi
        DOWNLOADER="wget"
    else
        DOWNLOADER="curl"
    fi
}

# Check if vcpkg is already installed
check_vcpkg() {
    # Check VCPKG_ROOT environment variable
    if [ -n "$VCPKG_ROOT" ] && [ -f "$VCPKG_ROOT/vcpkg" ] || [ -f "$VCPKG_ROOT/vcpkg.exe" ]; then
        echo "$VCPKG_ROOT"
        return 0
    fi
    
    # Check common installation locations
    COMMON_LOCATIONS="$HOME/vcpkg $HOME/.local/vcpkg $HOME/.vcpkg /opt/vcpkg /usr/local/vcpkg"
    for loc in $COMMON_LOCATIONS; do
        if [ -f "$loc/vcpkg" ] || [ -f "$loc/vcpkg.exe" ]; then
            echo "$loc"
            return 0
        fi
    done
    
    return 1
}

# Install vcpkg
# Returns vcpkg path via stdout (last line) if successful
install_vcpkg() {
    printf "\n%bChecking for vcpkg...%b\n" "$CYAN" "$NC" >&2
    
    # Check if vcpkg is already installed
    VCPKG_PATH=$(check_vcpkg)
    if [ -n "$VCPKG_PATH" ]; then
        printf "%bvcpkg found at: %s%b\n" "$GREEN" "$VCPKG_PATH" "$NC" >&2
        configure_vcpkg "$VCPKG_PATH" >&2
        # Return vcpkg path (to stdout, not stderr)
        echo "$VCPKG_PATH"
        return 0
    fi
    
    # Check if git is available (needed to clone vcpkg)
    if ! command -v git > /dev/null 2>&1; then
        printf "%bWarning: git is not installed. Skipping vcpkg installation.%b\n" "$YELLOW" "$NC" >&2
        printf "You can install vcpkg manually and configure it with: %bcpx config set-vcpkg-root <path>%b\n" "$CYAN" "$NC" >&2
        return 1
    fi
    
    # Automatically install vcpkg if not found
    printf "%bInstalling vcpkg...%b\n" "$CYAN" "$NC" >&2
    
    # Determine vcpkg installation directory
    VCPKG_INSTALL_DIR="$HOME/.local/vcpkg"
    if [ -n "$CPX_VCPKG_DIR" ]; then
        VCPKG_INSTALL_DIR="$CPX_VCPKG_DIR"
    fi
    
    printf "%bInstalling vcpkg to %s...%b\n" "$CYAN" "$VCPKG_INSTALL_DIR" "$NC" >&2
    
    # Remove existing directory if it exists (incomplete installation)
    if [ -d "$VCPKG_INSTALL_DIR" ]; then
        printf "%bRemoving existing directory...%b\n" "$YELLOW" "$NC" >&2
        rm -rf "$VCPKG_INSTALL_DIR"
    fi
    
    # Clone vcpkg
    printf "%bCloning vcpkg from GitHub...%b\n" "$CYAN" "$NC" >&2
    if ! git clone https://github.com/microsoft/vcpkg.git "$VCPKG_INSTALL_DIR" >&2; then
        printf "%bError: Failed to clone vcpkg%b\n" "$RED" "$NC" >&2
        return 1
    fi
    
    # Bootstrap vcpkg
    printf "%bBootstrapping vcpkg...%b\n" "$CYAN" "$NC" >&2
    cd "$VCPKG_INSTALL_DIR" || return 1
    
    OS=$(detect_os)
    if [ "$OS" = "windows" ]; then
        if ! ./bootstrap-vcpkg.bat >&2; then
            printf "%bError: Failed to bootstrap vcpkg%b\n" "$RED" "$NC" >&2
            cd - > /dev/null || true
            return 1
        fi
    else
        if ! ./bootstrap-vcpkg.sh >&2; then
            printf "%bError: Failed to bootstrap vcpkg%b\n" "$RED" "$NC" >&2
            cd - > /dev/null || true
            return 1
        fi
    fi
    
    cd - > /dev/null || true
    
    printf "%bSuccessfully installed vcpkg to %s%b\n" "$GREEN" "$VCPKG_INSTALL_DIR" "$NC" >&2
    
    # Configure cpx to use vcpkg
    configure_vcpkg "$VCPKG_INSTALL_DIR" >&2
    
    # Return vcpkg path (to stdout, not stderr)
    echo "$VCPKG_INSTALL_DIR"
    return 0
}

# Configure cpx to use vcpkg
configure_vcpkg() {
    VCPKG_PATH=$1
    
    # Check if cpx is in PATH
    if ! command -v "$BINARY_NAME" > /dev/null 2>&1; then
        INSTALL_DIR=$(get_install_dir)
        CPX_BINARY="$INSTALL_DIR/$BINARY_NAME"
    else
        CPX_BINARY=$(command -v "$BINARY_NAME")
    fi
    
    # Check if cpx binary exists
    if [ ! -f "$CPX_BINARY" ]; then
        printf "%bWarning: cpx binary not found. Cannot configure vcpkg automatically.%b\n" "$YELLOW" "$NC"
        printf "Run this after cpx is in your PATH: %bcpx config set-vcpkg-root %s%b\n" "$CYAN" "$VCPKG_PATH" "$NC"
        return 1
    fi
    
    # Configure vcpkg root
    printf "%bConfiguring cpx to use vcpkg...%b\n" "$CYAN" "$NC"
    if "$CPX_BINARY" config set-vcpkg-root "$VCPKG_PATH" 2>/dev/null; then
        printf "%bSuccessfully configured cpx to use vcpkg%b\n" "$GREEN" "$NC"
    else
        printf "%bWarning: Failed to configure cpx automatically.%b\n" "$YELLOW" "$NC"
        printf "Run this manually: %bcpx config set-vcpkg-root %s%b\n" "$CYAN" "$VCPKG_PATH" "$NC"
    fi
}

get_latest_version() {
    if [ "$DOWNLOADER" = "curl" ]; then
        VERSION=$(curl -sI "https://github.com/$REPO/releases/latest" | grep -i "location:" | sed 's/.*tag\///' | tr -d '\r\n')
    else
        VERSION=$(wget -qO- --server-response "https://github.com/$REPO/releases/latest" 2>&1 | grep -i "location:" | sed 's/.*tag\///' | tr -d '\r\n')
    fi
    
    if [ -z "$VERSION" ]; then
        VERSION="v1.1.5"
    fi
    echo "$VERSION"
}

# Check if BCR is already cloned
check_bcr() {
    # Check common installation locations
    BCR_LOCATIONS="$HOME/.local/bazel-central-registry $HOME/.cache/cpx/bazel-central-registry"
    for loc in $BCR_LOCATIONS; do
        if [ -d "$loc/modules" ]; then
            echo "$loc"
            return 0
        fi
    done
    
    return 1
}

# Clone Bazel Central Registry
install_bcr() {
    printf "\n%bChecking for Bazel Central Registry...%b\n" "$CYAN" "$NC" >&2
    
    # Check if BCR is already cloned
    BCR_PATH=$(check_bcr)
    if [ -n "$BCR_PATH" ]; then
        printf "%bBCR found at: %s%b\n" "$GREEN" "$BCR_PATH" "$NC" >&2
        configure_bcr "$BCR_PATH" >&2
        echo "$BCR_PATH"
        return 0
    fi
    
    # Check if git is available
    if ! command -v git > /dev/null 2>&1; then
        printf "%bWarning: git is not installed. Skipping BCR installation.%b\n" "$YELLOW" "$NC" >&2
        printf "You can install BCR manually and configure it with: %bcpx config set-bcr-root <path>%b\n" "$CYAN" "$NC" >&2
        return 1
    fi
    
    # Clone BCR
    BCR_INSTALL_DIR="$HOME/.local/bazel-central-registry"
    printf "%bCloning Bazel Central Registry to %s...%b\n" "$CYAN" "$BCR_INSTALL_DIR" "$NC" >&2
    printf "%b(This may take a while - the registry is large)%b\n" "$YELLOW" "$NC" >&2
    
    # Remove existing directory if incomplete
    if [ -d "$BCR_INSTALL_DIR" ] && [ ! -d "$BCR_INSTALL_DIR/modules" ]; then
        rm -rf "$BCR_INSTALL_DIR"
    fi
    
    # Clone with depth 1 (shallow clone)
    if ! git clone --depth 1 https://github.com/bazelbuild/bazel-central-registry.git "$BCR_INSTALL_DIR" >&2; then
        printf "%bWarning: Failed to clone BCR. You can clone it manually later.%b\n" "$YELLOW" "$NC" >&2
        printf "  Run: git clone https://github.com/bazelbuild/bazel-central-registry.git ~/.local/bazel-central-registry\n" >&2
        printf "  Then: cpx config set-bcr-root ~/.local/bazel-central-registry\n" >&2
        return 1
    fi
    
    printf "%bSuccessfully cloned BCR to %s%b\n" "$GREEN" "$BCR_INSTALL_DIR" "$NC" >&2
    configure_bcr "$BCR_INSTALL_DIR" >&2
    echo "$BCR_INSTALL_DIR"
    return 0
}

# Configure cpx to use BCR
configure_bcr() {
    BCR_PATH=$1
    
    # Check if cpx is in PATH
    if ! command -v "$BINARY_NAME" > /dev/null 2>&1; then
        INSTALL_DIR=$(get_install_dir)
        CPX_BINARY="$INSTALL_DIR/$BINARY_NAME"
    else
        CPX_BINARY=$(command -v "$BINARY_NAME")
    fi
    
    # Check if cpx binary exists
    if [ ! -f "$CPX_BINARY" ]; then
        printf "%bWarning: cpx binary not found. Cannot configure BCR automatically.%b\n" "$YELLOW" "$NC"
        printf "Run this after cpx is in your PATH: %bcpx config set-bcr-root %s%b\n" "$CYAN" "$BCR_PATH" "$NC"
        return 1
    fi
    
    # Configure BCR root
    printf "%bConfiguring cpx to use BCR...%b\n" "$CYAN" "$NC"
    if "$CPX_BINARY" config set-bcr-root "$BCR_PATH" 2>/dev/null; then
        printf "%bSuccessfully configured cpx to use BCR%b\n" "$GREEN" "$NC"
    else
        printf "%bWarning: Failed to configure BCR automatically.%b\n" "$YELLOW" "$NC"
        printf "Run this manually: %bcpx config set-bcr-root %s%b\n" "$CYAN" "$BCR_PATH" "$NC"
    fi
}

download_binary() {
    OS=$1
    ARCH=$2
    VERSION=$3
    
    if [ "$OS" = "unknown" ] || [ "$ARCH" = "unknown" ]; then
        printf "%bError: Unsupported platform: %s/%s%b\n" "$RED" "$OS" "$ARCH" "$NC"
        exit 1
    fi
    
    BINARY_NAME_PLATFORM="$BINARY_NAME-$OS-$ARCH"
    if [ "$OS" = "windows" ]; then
        BINARY_NAME_PLATFORM="${BINARY_NAME_PLATFORM}.exe"
    fi
    
    DOWNLOAD_URL="https://github.com/$REPO/releases/download/$VERSION/$BINARY_NAME_PLATFORM"
    
    printf "%bDownloading %s from %s...%b\n" "$CYAN" "$BINARY_NAME_PLATFORM" "$DOWNLOAD_URL" "$NC"
    
    INSTALL_DIR=$(get_install_dir)
    TARGET_PATH="$INSTALL_DIR/$BINARY_NAME"
    
    # Create install directory if it doesn't exist
    mkdir -p "$INSTALL_DIR"
    
    if [ "$DOWNLOADER" = "curl" ]; then
        if ! curl -fSL "$DOWNLOAD_URL" -o "$TARGET_PATH"; then
            printf "%bError: Failed to download binary%b\n" "$RED" "$NC"
            exit 1
        fi
    else
        if ! wget -q "$DOWNLOAD_URL" -O "$TARGET_PATH"; then
            printf "%bError: Failed to download binary%b\n" "$RED" "$NC"
            exit 1
        fi
    fi
    
    chmod +x "$TARGET_PATH"
    
    printf "%bSuccessfully installed %s to %s%b\n" "$GREEN" "$BINARY_NAME" "$TARGET_PATH" "$NC"
    
    # Check if binary is in PATH
    if ! command -v "$BINARY_NAME" > /dev/null 2>&1; then
        printf "%bWarning: %s is not in your PATH.%b\n" "$YELLOW" "$BINARY_NAME" "$NC"
        printf "Add this to your shell profile (.bashrc, .zshrc, etc.):\n"
        printf "  export PATH=\"\$PATH:%s\"\n" "$INSTALL_DIR"
    else
        printf "%b%s is ready to use!%b\n" "$GREEN" "$BINARY_NAME" "$NC"
        printf "Run '%b%s --version%b' to verify installation.\n" "$CYAN" "$BINARY_NAME" "$NC"
    fi
}

# Get config directory path
get_config_dir() {
    OS=$1
    if [ "$OS" = "windows" ]; then
        if [ -n "$APPDATA" ]; then
            echo "$APPDATA/cpx"
        else
            echo "$HOME/AppData/Roaming/cpx"
        fi
    else
        echo "$HOME/.config/cpx"
    fi
}

# Create or update cpx config file
create_config_file() {
    OS=$1
    VCPKG_PATH=$2
    
    CONFIG_DIR=$(get_config_dir "$OS")
    CONFIG_FILE="$CONFIG_DIR/config.yaml"
    
    printf "\n%bSetting up cpx config file...%b\n" "$CYAN" "$NC" >&2
    
    # Create config directory
    if ! mkdir -p "$CONFIG_DIR" 2>/dev/null; then
        printf "%bWarning: Failed to create config directory: %s%b\n" "$YELLOW" "$CONFIG_DIR" "$NC" >&2
        return 1
    fi
    
    # If config file exists and vcpkg path is provided, update it
    if [ -f "$CONFIG_FILE" ] && [ -n "$VCPKG_PATH" ]; then
        # Update existing config file with vcpkg path
        if command -v sed > /dev/null 2>&1; then
            # Use sed to update vcpkg_root if it's empty or update existing value
            if grep -q "^vcpkg_root:" "$CONFIG_FILE"; then
                # Escape slashes in path for sed
                ESCAPED_PATH=$(echo "$VCPKG_PATH" | sed 's/\//\\\//g')
                sed -i.bak "s/^vcpkg_root:.*/vcpkg_root: \"$ESCAPED_PATH\"/" "$CONFIG_FILE" 2>/dev/null || \
                sed -i '' "s/^vcpkg_root:.*/vcpkg_root: \"$ESCAPED_PATH\"/" "$CONFIG_FILE" 2>/dev/null
                rm -f "${CONFIG_FILE}.bak" 2>/dev/null || true
                printf "%bUpdated config file with vcpkg path: %s%b\n" "$GREEN" "$VCPKG_PATH" "$NC" >&2
            else
                # Add vcpkg_root if it doesn't exist
                echo "vcpkg_root: \"$VCPKG_PATH\"" >> "$CONFIG_FILE"
                printf "%bAdded vcpkg path to config file: %s%b\n" "$GREEN" "$VCPKG_PATH" "$NC" >&2
            fi
        else
            # Fallback: recreate file with vcpkg path
            printf "vcpkg_root: \"%s\"\n" "$VCPKG_PATH" > "$CONFIG_FILE"
            printf "%bUpdated config file with vcpkg path: %s%b\n" "$GREEN" "$VCPKG_PATH" "$NC" >&2
        fi
        return 0
    fi
    
    # Create new config file
    if [ ! -f "$CONFIG_FILE" ]; then
        printf "%bCreating cpx config file...%b\n" "$CYAN" "$NC" >&2
        
        if [ -n "$VCPKG_PATH" ]; then
            # Create config file with vcpkg path
            printf "vcpkg_root: \"%s\"\n" "$VCPKG_PATH" > "$CONFIG_FILE"
        else
            # Create config file with empty vcpkg_root
            printf "vcpkg_root: \"\"\n" > "$CONFIG_FILE"
        fi
        
        if [ -f "$CONFIG_FILE" ]; then
            printf "%bCreated config file: %s%b\n" "$GREEN" "$CONFIG_FILE" "$NC" >&2
            return 0
        else
            printf "%bWarning: Failed to create config file%b\n" "$YELLOW" "$NC" >&2
            return 1
        fi
    fi
    
    # Config file already exists, nothing to do
    printf "%bConfig file already exists: %s%b\n" "$GREEN" "$CONFIG_FILE" "$NC" >&2
    
    return 0
}

# Install dockerfiles to config directory
install_dockerfiles() {
    OS=$1
    
    CONFIG_DIR=$(get_config_dir "$OS")
    DOCKERFILES_DIR="$CONFIG_DIR/dockerfiles"
    
    # Check if dockerfiles already exist
    if [ -d "$DOCKERFILES_DIR" ] && [ -f "$DOCKERFILES_DIR/Dockerfile.linux-amd64" ]; then
        printf "%bDockerfiles already installed.%b\n" "$GREEN" "$NC" >&2
        return 0
    fi
    
    printf "%bInstalling dockerfiles...%b\n" "$CYAN" "$NC" >&2
    
    # Create dockerfiles directory
    if ! mkdir -p "$DOCKERFILES_DIR" 2>/dev/null; then
        printf "%bWarning: Failed to create dockerfiles directory: %s%b\n" "$YELLOW" "$DOCKERFILES_DIR" "$NC" >&2
        return 1
    fi
    
    # List of dockerfiles to download
    DOCKERFILES=(
        "Dockerfile.linux-amd64"
        "Dockerfile.linux-amd64-musl"
        "Dockerfile.linux-arm64"
        "Dockerfile.linux-arm64-musl"
        "Dockerfile.windows-amd64"
        "Dockerfile.macos-amd64"
        "Dockerfile.macos-arm64"
        "cpx.ci.example"
        "README.md"
    )
    
    # Download each dockerfile from GitHub
    BASE_URL="https://raw.githubusercontent.com/$REPO/main/dockerfiles"
    FAILED=0
    
    for dockerfile in "${DOCKERFILES[@]}"; do
        DEST_PATH="$DOCKERFILES_DIR/$dockerfile"
        DOWNLOAD_URL="$BASE_URL/$dockerfile"
        
        if [ "$DOWNLOADER" = "curl" ]; then
            if ! curl -fsSL "$DOWNLOAD_URL" -o "$DEST_PATH" 2>/dev/null; then
                printf "%bWarning: Failed to download %s%b\n" "$YELLOW" "$dockerfile" "$NC" >&2
                FAILED=1
            fi
        else
            if ! wget -q "$DOWNLOAD_URL" -O "$DEST_PATH" 2>/dev/null; then
                printf "%bWarning: Failed to download %s%b\n" "$YELLOW" "$dockerfile" "$NC" >&2
                FAILED=1
            fi
        fi
    done
    
    if [ $FAILED -eq 0 ]; then
        printf "%bSuccessfully installed dockerfiles to %s%b\n" "$GREEN" "$DOCKERFILES_DIR" "$NC" >&2
        return 0
    else
        printf "%bWarning: Some dockerfiles failed to download. You can download them later with 'cpx upgrade'.%b\n" "$YELLOW" "$NC" >&2
        return 1
    fi
}

main() {
    print_banner
    
    check_dependencies
    
    OS=$(detect_os)
    ARCH=$(detect_arch)
    VERSION=$(get_latest_version)
    
    printf "%bDetected: %s/%s%b\n" "$CYAN" "$OS" "$ARCH" "$NC"
    printf "%bLatest version: %s%b\n" "$CYAN" "$VERSION" "$NC"
    printf "\n"
    
    download_binary "$OS" "$ARCH" "$VERSION"
    
    # Always create config file first (before vcpkg check)
    printf "\n"
    create_config_file "$OS" "" || true
    
    # Install dockerfiles
    install_dockerfiles "$OS" || true
    
    # Try to install/configure vcpkg (non-fatal if it fails)
    # Skip on Windows unless in Git Bash/MSYS2
    VCPKG_PATH=""
    if [ "$OS" != "windows" ] || [ -n "$MSYSTEM" ]; then
        printf "\n"
        # First check if vcpkg already exists (suppress errors with || true)
        EXISTING_VCPKG=$(check_vcpkg 2>/dev/null || echo "")
        if [ -n "$EXISTING_VCPKG" ]; then
            VCPKG_PATH="$EXISTING_VCPKG"
            printf "%bFound existing vcpkg at: %s%b\n" "$GREEN" "$VCPKG_PATH" "$NC"
            configure_vcpkg "$VCPKG_PATH" || true
            # Update config file with vcpkg path
            create_config_file "$OS" "$VCPKG_PATH" || true
        else
            # Install vcpkg automatically
            printf "%bInstalling vcpkg...%b\n" "$CYAN" "$NC"
            # Capture all output (stderr messages + stdout path)
            OUTPUT=$(install_vcpkg 2>&1 || echo "INSTALL_FAILED")
            INSTALL_EXIT=$?
            # Display all output except the last line (which is the path)
            if [ -n "$OUTPUT" ] && [ "$OUTPUT" != "INSTALL_FAILED" ]; then
                echo "$OUTPUT" | sed '$d'
            fi
            # Get the last line which should be the vcpkg path
            if [ "$OUTPUT" != "INSTALL_FAILED" ]; then
                VCPKG_PATH=$(echo "$OUTPUT" | tail -1)
            fi
            # Only use the path if it looks valid and installation succeeded
            if [ $INSTALL_EXIT -eq 0 ] && [ -n "$VCPKG_PATH" ] && [ -d "$VCPKG_PATH" ]; then
                # Update config file with vcpkg path
                create_config_file "$OS" "$VCPKG_PATH" || true
            elif [ $INSTALL_EXIT -ne 0 ] || [ "$OUTPUT" = "INSTALL_FAILED" ]; then
                printf "%bWarning: vcpkg installation failed. You can install it manually later.%b\n" "$YELLOW" "$NC"
                printf "  Run: %bcpx config set-vcpkg-root <path>%b\n" "$CYAN" "$NC"
            fi
        fi
    else
        printf "\n%bNote: vcpkg installation on Windows requires manual setup.%b\n" "$YELLOW" "$NC"
        printf "After installing vcpkg, run: %bcpx config set-vcpkg-root <path>%b\n" "$CYAN" "$NC"
    fi
    
    # Install BCR for Bazel support (non-fatal if it fails)
    if [ "$OS" != "windows" ] || [ -n "$MSYSTEM" ]; then
        # Check if BCR already exists
        EXISTING_BCR=$(check_bcr 2>/dev/null || echo "")
        if [ -n "$EXISTING_BCR" ]; then
            printf "%bFound existing BCR at: %s%b\n" "$GREEN" "$EXISTING_BCR" "$NC"
            configure_bcr "$EXISTING_BCR" || true
        else
            # Install BCR
            install_bcr 2>&1 | sed '$d' || true
        fi
    fi
}

main
