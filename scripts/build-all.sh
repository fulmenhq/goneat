#!/bin/bash
# Cross-platform build script for goneat
# Builds binaries for all supported platforms from a single machine

set -euo pipefail

# shellcheck source=scripts/release-platforms.sh
source "$(dirname "${BASH_SOURCE[0]}")/release-platforms.sh"
export GOTOOLCHAIN="${GOTOOLCHAIN:-go1.26.6}"

# Get version from VERSION file (already contains 'v' prefix)
VERSION=$(cat VERSION)
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(git rev-parse HEAD 2>/dev/null || echo "unknown")
echo "🔨 Building goneat $VERSION for all platforms..."
echo "   Build time: $BUILD_TIME"
echo "   Git commit: ${GIT_COMMIT:0:8}"

# Ensure embedded assets are synced from SSOT
echo "📦 Syncing embedded assets (templates/, schemas/)..."
env -u GOOS -u GOARCH make -s embed-assets verify-embeds

# Generators above must run on the host, not a cross-compilation target.
HOST_OS=$(env -u GOOS -u GOARCH go env GOHOSTOS)
HOST_ARCH=$(env -u GOOS -u GOARCH go env GOHOSTARCH)

# Create build directory
mkdir -p bin

echo "📦 Building for ${#RELEASE_TARGETS[@]} platforms..."

for target in "${RELEASE_TARGETS[@]}"; do
	GOOS=${target%/*}
	GOARCH=${target#*/}

	echo "🏗️  Building for $GOOS/$GOARCH..."

	# Set binary extension for Windows
	EXT=""
	if [ "$GOOS" = "windows" ]; then
		EXT=".exe"
	fi

	# Build with version information embedded via ldflags
	# Must match pkg/buildinfo/buildinfo.go variable paths
	#
	# No libc linkage: Linux artifacts must work with both glibc and musl.
	env CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build \
		-ldflags "\
			-X 'github.com/fulmenhq/goneat/pkg/buildinfo.BinaryVersion=$VERSION' \
			-X 'github.com/fulmenhq/goneat/pkg/buildinfo.BuildTime=$BUILD_TIME' \
			-X 'github.com/fulmenhq/goneat/pkg/buildinfo.GitCommit=$GIT_COMMIT'" \
		-o "bin/goneat-$GOOS-$GOARCH$EXT" \
		.

	# Verify the binary was created and is executable
	if [ -f "bin/goneat-$GOOS-$GOARCH$EXT" ]; then
		echo "✅ Built bin/goneat-$GOOS-$GOARCH$EXT"

		# For Linux, assert no dynamic libc linkage (prevents musl container failures)
		if [ "$GOOS" = "linux" ]; then
			if file "bin/goneat-$GOOS-$GOARCH$EXT" | grep -q "dynamically linked"; then
				echo "❌ Linux binary is dynamically linked (glibc/musl incompatibility risk)"
				file "bin/goneat-$GOOS-$GOARCH$EXT"
				exit 1
			fi
		fi

		# Never execute a foreign target or silently pass a failing native binary.
		if [ "$GOOS/$GOARCH" = "$HOST_OS/$HOST_ARCH" ]; then
			"./bin/goneat-$GOOS-$GOARCH$EXT" version >/dev/null
			echo "🧪 Binary functional: $GOOS/$GOARCH"
		fi
	else
		echo "❌ Build failed: $GOOS/$GOARCH"
		exit 1
	fi
done

echo ""
echo "🎉 All builds completed successfully!"
echo ""
echo "📦 Build artifacts:"
ls -lh bin/

echo ""
echo "📊 Build summary:"
echo "   Platforms: ${#RELEASE_TARGETS[@]}"
echo "   Version: $VERSION"
echo "   Total binaries in bin/: $(find bin/ -maxdepth 1 -type f | wc -l)"

echo ""
echo "🚀 Ready for distribution!"
echo "   Upload to: https://github.com/fulmenhq/goneat/releases"
echo ""
