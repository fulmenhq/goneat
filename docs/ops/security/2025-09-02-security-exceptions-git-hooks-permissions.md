# Security Exceptions

This document records security findings that have been reviewed and deemed acceptable security trade-offs.

## Git Hook Permissions (cmd/hooks.go)

**Issue**: Gosec flags G302/G306 for file permissions exceeding 0600

**Lines**: 271, 361, 389, 420

**Justification**: Git hooks require executable permissions (0700/755) to function properly. These are legitimate executable scripts that users install in their `.git/hooks/` directory.

**Risk Assessment**: Low - These files are:

1. Only written to the user's own `.git/hooks/` directory
2. Only executable by the user who installed them
3. Created with user-controlled content (pre-commit/pre-push hooks)
4. Essential for git workflow functionality

**Mitigation**: Files include `#nosec G302/G306` suppression comments with explanatory notes.

**Status**: Accepted - Required for core functionality

---

_Last Updated: 2025-09-02_
_Reviewed By: Code Scout AI Agent_
