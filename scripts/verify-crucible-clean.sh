#!/bin/bash
# Verify that all crucible SSOT sources are clean (not dirty)
# This prevents pushing goneat releases when crucible content has uncommitted changes
#
# Usage:
#   ./scripts/verify-crucible-clean.sh
#   make verify-crucible-clean

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PROVENANCE_FILE="$REPO_ROOT/.goneat/ssot/provenance.json"

# Color output helpers
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Check if provenance file exists
if [ ! -f "$PROVENANCE_FILE" ]; then
    echo -e "${YELLOW}⚠️  Warning: Provenance file not found at $PROVENANCE_FILE${NC}"
    echo "Skipping crucible clean check (no SSOT synchronization detected)"
    exit 0
fi

# Check if jq is available
if ! command -v jq &> /dev/null; then
    echo -e "${RED}❌ Error: jq is required to parse provenance.json${NC}"
    echo "Install jq: brew install jq (macOS) or apt install jq (Linux)"
    exit 1
fi

# Validate provenance.json against schema
echo -e "${BLUE}🔍 Validating provenance.json against schema...${NC}"
GONEAT_BIN="$REPO_ROOT/dist/goneat"
if [ -f "$GONEAT_BIN" ]; then
    # Run validation and suppress output, checking only exit code
    if ! "$GONEAT_BIN" validate "$PROVENANCE_FILE" --log-level error >/dev/null 2>&1; then
        echo -e "${RED}❌ Error: Provenance file is invalid or does not conform to schema${NC}"
        echo "Run: goneat validate $PROVENANCE_FILE"
        echo "This may indicate corrupted SSOT metadata or schema mismatch"
        exit 1
    fi
    echo -e "${GREEN}✅ Provenance file is valid${NC}"
else
    echo -e "${YELLOW}⚠️  Warning: goneat binary not found, skipping schema validation${NC}"
fi

echo ""
echo -e "${BLUE}🔍 Checking crucible SSOT sources for uncommitted changes...${NC}"
echo ""

# Parse provenance.json and check for dirty sources
DIRTY_SOURCES=$(jq -r '.sources[] | select(.dirty == true) | "\(.name)|\(.commit)|\(.dirty_reason)"' "$PROVENANCE_FILE")

if [ -z "$DIRTY_SOURCES" ]; then
    echo -e "${GREEN}✅ All crucible sources are clean (no uncommitted changes)${NC}"
    echo ""
    # Show source status for transparency
    echo "Source status:"
    jq -r '.sources[] | "  • \(.name): \(.commit[0:8]) (\(.version)) - clean"' "$PROVENANCE_FILE"
    exit 0
fi

# Found dirty sources - report and fail
echo -e "${RED}❌ Push blocked: Crucible sources have uncommitted changes${NC}"
echo ""
echo "The following SSOT sources are dirty:"
echo ""

while IFS='|' read -r name commit reason; do
    echo -e "${YELLOW}  ⚠️  Source: ${name}${NC}"
    echo "     Commit: ${commit:0:8}"
    echo "     Reason: $reason"
    echo ""
done <<< "$DIRTY_SOURCES"

echo -e "${RED}Why this matters:${NC}"
echo "  Pushing goneat with dirty crucible sources can lead to:"
echo "  • Inconsistent documentation across repositories"
echo "  • Schema/template mismatches between goneat and crucible"
echo "  • Difficulty tracking which crucible version was used"
echo ""
echo -e "${BLUE}To resolve:${NC}"
echo "  1. Go to the crucible repository and commit/push your changes:"
echo "     cd ../crucible"
echo "     git status"
echo "     git add ."
echo "     git commit -m 'your changes'"
echo "     git push"
echo ""
echo "  2. Re-sync goneat with clean crucible:"
echo "     cd $REPO_ROOT"
echo "     make sync-ssot"
echo ""
echo "  3. Commit the updated provenance.json:"
echo "     git add .goneat/ssot/provenance.json"
echo "     git commit -m 'chore: sync with clean crucible sources'"
echo ""
echo "  4. Retry your push:"
echo "     git push"
echo ""

exit 1
