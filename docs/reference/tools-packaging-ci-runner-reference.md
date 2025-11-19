

# 🛠️ CI Runner Utilities: Non-Sudo Dependency Management

This document provides recipes for ensuring common development utilities, particularly those found in **GNU Coreutils**, are available in CI/development environments without requiring root/administrator (e.g., `sudo`) privileges. This is crucial for pipeline speed and security.

-----

## 📦 Package Manager Artifacts (No `sudo` required)

While system package managers like `apt` or `brew` often require initial setup with `sudo`, we can leverage their **untar-anywhere** installation methods to set them up within a user-owned directory (like `$HOME` or the CI workspace).

### Recipe 1: Installing Homebrew Locally (macOS/Linux)

This process downloads the Homebrew repository directly into a user-local path and uses `tar` and `curl` (common in CI runners) to install it, bypassing the `sudo` requirement of the standard installer.

| Platform | Dependencies | Notes |
| :--- | :--- | :--- |
| **macOS/Linux** | `curl`, `tar`, `bash` | Homebrew will build packages from source if pre-compiled **bottles** aren't available for the non-default prefix. |

```bash
#!/bin/bash
# RECIPE 1: Non-Sudo Homebrew Installation

# --- Configuration ---
# Set the desired non-default installation prefix
export HOMEBREW_PREFIX="$HOME/homebrew-local"
# Ensure the directory exists
mkdir -p "$HOMEBREW_PREFIX"

# --- Install Homebrew Core ---
# Download and extract Homebrew directly into the prefix
echo "Installing Homebrew to $HOMEBREW_PREFIX..."
curl -L https://github.com/Homebrew/brew/tarball/master | tar xz --strip 1 -C "$HOMEBREW_PREFIX"

# --- Configure Environment ---
# Export the necessary variables to teach the shell where Brew lives
export PATH="$HOMEBREW_PREFIX/bin:$PATH"
export HOMEBREW_CELLAR="$HOMEBREW_PREFIX/Cellar"
export HOMEBREW_REPOSITORY="$HOMEBREW_PREFIX/Homebrew"

# --- Verify Installation ---
echo "Homebrew installed successfully at $(which brew)"
brew --version

# --- Install coreutils for 'timeout' ---
# On macOS, the GNU version of 'timeout' is installed as 'gtimeout'
brew install coreutils

# --- Create Global Symlink for 'timeout' ---
# Create a symlink to 'gtimeout' so it can be called as 'timeout'
# This is safe because $HOMEBREW_PREFIX/bin is prioritized in $PATH
ln -s "$(brew --prefix coreutils)/bin/gtimeout" "$HOMEBREW_PREFIX/bin/timeout"

# --- Test ---
echo "Testing 'timeout' availability..."
timeout 3s sleep 5 && echo "Command succeeded" || echo "Command timed out"
```

-----

-----

## 📦 Bundled `cpanminus` (Perl Module Installer)

Some CI workflows still rely on Perl modules. `cpanminus` allows module installation into `$HOME` without touching the system Perl or requiring sudo.

### Recipe 2: Install cpanminus and Modules

```bash
#!/bin/bash
# Install cpanminus locally and fetch Perl modules

# Set local lib paths
export PERL5LIB="$HOME/perl5/lib/perl5"
export PERL_LOCAL_LIB_ROOT="$HOME/perl5"
export PERL_MB_OPT="--install_base '$HOME/perl5'"
export PERL_MM_OPT="INSTALL_BASE=$HOME/perl5"

# Install cpanminus (curl fallback)
if ! command -v cpanm >/dev/null; then
  curl -L https://cpanmin.us | perl - App::cpanminus --self-upgrade --local-lib="$HOME/perl5"
fi

# Install modules without sudo
cpanm --local-lib="$HOME/perl5" JSON::MaybeXS

# Verify
perl -MJSON::MaybeXS -e 'print JSON::MaybeXS::encode_json({ok=>1})'
```

Use Cases:
- Coverage tooling for legacy Perl components
- Template Toolkit or JSON parsing utilities during build steps

-----

## 🔨 Essential GNU Core Utilities for Developers

GNU Coreutils provide consistent behavior across Linux/macOS runners (BSD utilities often diverge). Installing them ensures scripts behave identically.

| Utility | Description | Use Case Example |
| :--- | :--- | :--- |
| **`timeout`** | Runs a command with a time limit. | Enforcing test suite runtimes; preventing hanging CI steps. |
| **`gdate`** | GNU version of `date`. | Consistent date formatting (e.g., RFC 3339 timestamps for logs). |
| **`gshuf`** | Generates random permutations (shuffle). | Creating randomized input for fuzz testing or unique temporary names. |
| **`gsort`** | GNU version of `sort`. | Consistent sorting behavior for file diffs and deterministic outputs. |
| **`greadlink`** | Canonicalizes file paths (GNU `readlink`). | Resolving symbolic links robustly to find the true binary location. |

-----

## 💻 Tool Artifacts: Compile from Source

The most robust, truly cross-platform solution (Linux, macOS, and even Windows via WSL/MinGW/Cygwin environments) is compiling the target utility from source directly into your workspace. This recipe provides a template for building a utility from the GNU Coreutils package.

### Recipe 2: Compile `timeout` from Coreutils Source

This script downloads and builds the Coreutils package, installing it into a user-local directory. This requires `gcc` (or equivalent C compiler) and `make` to be available on the runner.

| Platform | Dependencies | Notes |
| :--- | :--- | :--- |
| **Linux/macOS** | `curl` or `wget`, `tar`, `make`, `gcc` | Assumes a standard Unix-like build toolchain is available on the runner image. |
| **Windows** | Requires a Unix-like layer (WSL, Git Bash, MinGW, MSYS2) where the above dependencies are available. | If a static binary is required, **cross-compilation** on a Linux host is generally more reliable than building directly on Windows. |

```bash
#!/bin/bash
# RECIPE 2: Compile GNU Coreutils from Source (No-Sudo)

# --- Configuration ---
# Coreutils source version
COREUTILS_VERSION="9.5" # Update as needed
# Installation directory (user-owned)
export LOCAL_PREFIX="$HOME/ci-tools/coreutils-bin"
# Source archive URL
COREUTILS_URL="https://ftp.gnu.org/gnu/coreutils/coreutils-$COREUTILS_VERSION.tar.xz"

# --- Setup ---
mkdir -p "$LOCAL_PREFIX/src"
cd "$LOCAL_PREFIX/src"

# --- Download Source (Platform Neutralized) ---
# Use 'curl' if 'wget' is not available
if command -v wget &> /dev/null; then
    echo "Using wget to download source."
    wget -qO- "$COREUTILS_URL" | tar xJ
elif command -v curl &> /dev/null; then
    echo "Using curl to download source."
    curl -Ls "$COREUTILS_URL" | tar xJ
else
    echo "Error: Neither wget nor curl found. Cannot download source."
    exit 1
fi

# Navigate into the extracted source directory
cd "coreutils-$COREUTILS_VERSION"

# --- Build and Install ---
# The --prefix flag ensures files are installed to the user-owned directory
echo "Configuring and compiling Coreutils..."
./configure --prefix="$LOCAL_PREFIX"
make -j$(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4) # Use parallel make

# Install the binaries to the user-local prefix (NO 'sudo' needed)
make install

# --- Finalize ---
# Add the new binary path to the CI runner's environment
export PATH="$LOCAL_PREFIX/bin:$PATH"

# --- Test ---
echo "GNU Coreutils 'timeout' is installed at: $(which timeout)"
timeout 1s sleep 3 && echo "Command succeeded" || echo "Command timed out"
```
