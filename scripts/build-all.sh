#!/bin/bash
# Cross-platform build script for goneat
# Builds binaries for all supported platforms from a single machine

set -e

# Get version from VERSION file (already contains 'v' prefix)
VERSION=$(cat VERSION)
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(git rev-parse HEAD 2>/dev/null || echo "unknown")
echo "🔨 Building goneat $VERSION for all platforms..."
echo "   Build time: $BUILD_TIME"
echo "   Git commit: ${GIT_COMMIT:0:8}"

# Ensure embedded assets are synced from SSOT
echo "📦 Syncing embedded assets (templates/, schemas/)..."
make -s embed-assets

# Define build targets
TARGETS=(
	"linux/amd64"
	"linux/arm64"
	"darwin/amd64"
	"darwin/arm64"
	"windows/amd64"
)

# Create build directory
mkdir -p bin

echo "📦 Building for ${#TARGETS[@]} platforms..."

for target in "${TARGETS[@]}"; do
	GOOS=$(echo $target | cut -d/ -f1)
	GOARCH=$(echo $target | cut -d/ -f2)

	echo "🏗️  Building for $GOOS/$GOARCH..."

	# Set binary extension for Windows
	EXT=""
	if [ "$GOOS" = "windows" ]; then
		EXT=".exe"
	fi

	# Build with version information embedded via ldflags
	# Must match pkg/buildinfo/buildinfo.go variable paths
	#
	# Linux release artifacts must run in both glibc and musl environments (e.g. Alpine).
	# Build Linux binaries with CGO disabled to avoid libc linkage.
	if [ "$GOOS" = "linux" ]; then
		CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build \
			-ldflags "\
				-X 'github.com/fulmenhq/goneat/pkg/buildinfo.BinaryVersion=$VERSION' \
				-X 'github.com/fulmenhq/goneat/pkg/buildinfo.BuildTime=$BUILD_TIME' \
				-X 'github.com/fulmenhq/goneat/pkg/buildinfo.GitCommit=$GIT_COMMIT'" \
			-o "bin/goneat-$GOOS-$GOARCH$EXT" \
			.
	else
		GOOS=$GOOS GOARCH=$GOARCH go build \
			-ldflags "\
				-X 'github.com/fulmenhq/goneat/pkg/buildinfo.BinaryVersion=$VERSION' \
				-X 'github.com/fulmenhq/goneat/pkg/buildinfo.BuildTime=$BUILD_TIME' \
				-X 'github.com/fulmenhq/goneat/pkg/buildinfo.GitCommit=$GIT_COMMIT'" \
			-o "bin/goneat-$GOOS-$GOARCH$EXT" \
			.
	fi

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

		# Quick test to ensure binary works (native platforms only)
		if "./bin/goneat-$GOOS-$GOARCH$EXT" version >/dev/null 2>&1; then
			echo "🧪 Binary functional: $GOOS/$GOARCH"
		else
			echo "⚠️  Binary test failed: $GOOS/$GOARCH"
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
echo "   Platforms: ${#TARGETS[@]}"
echo "   Version: $VERSION"
echo "   Total binaries: $(ls bin/ | wc -l)"

echo ""
echo "🚀 Ready for distribution!"
echo "   Upload to: https://github.com/fulmenhq/goneat/releases"
echo ""
