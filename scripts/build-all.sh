#!/bin/bash
# Cross-platform build script for goneat
# Builds binaries for all supported platforms from a single machine

set -e

# Get version from VERSION file (already contains 'v' prefix)
VERSION=$(cat VERSION)
echo "🔨 Building goneat $VERSION for all platforms..."

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

	# Build with version information embedded
	GOOS=$GOOS GOARCH=$GOARCH go build \
		-ldflags "-X 'main.Version=$VERSION'" \
		-o "bin/goneat-$GOOS-$GOARCH$EXT" \
		.

	# Verify the binary was created and is executable
	if [ -f "bin/goneat-$GOOS-$GOARCH$EXT" ]; then
		echo "✅ Built bin/goneat-$GOOS-$GOARCH$EXT"

		# Quick test to ensure binary works
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
