package ssot

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fulmenhq/goneat/pkg/config"
	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

// ClonedRepo represents a cloned repository (cached under goneat home).
type ClonedRepo struct {
	Path   string
	Cached bool
}

const ssotCacheDirName = "cache/ssot"

// CloneRepository clones the given GitHub repo/ref (or file:// URL) into the SSOT cache.
// Subsequent calls reuse the cached clone keyed by repo+ref, fetching latest updates when possible.
func CloneRepository(repo, ref string) (*ClonedRepo, error) {
	if repo == "" {
		return nil, errors.New("repo cannot be empty")
	}
	if ref == "" {
		return nil, errors.New("ref cannot be empty")
	}

	cacheDir, err := ensureSSOTCacheDir()
	if err != nil {
		return nil, err
	}

	cacheKey := hashRepoRef(repo, ref)
	targetPath := filepath.Join(cacheDir, cacheKey)

	// Attempt to reuse existing clone
	repository, cached, err := openOrCloneRepo(repo, ref, targetPath)
	if err != nil {
		return nil, err
	}

	hash, err := resolveRefHash(repository, ref)
	if err != nil {
		if !cached {
			_ = os.RemoveAll(targetPath)
		}
		return nil, err
	}

	if err := checkoutHash(repository, hash); err != nil {
		if !cached {
			_ = os.RemoveAll(targetPath)
		}
		return nil, fmt.Errorf("failed to checkout %s: %w", ref, err)
	}

	return &ClonedRepo{
		Path:   targetPath,
		Cached: cached,
	}, nil
}

// ensureSSOTCacheDir creates the ssot cache directory under goneat home.
func ensureSSOTCacheDir() (string, error) {
	home, err := config.EnsureGoneatHome()
	if err != nil {
		return "", err
	}

	cacheDir := filepath.Join(home, ssotCacheDirName)
	if err := os.MkdirAll(cacheDir, 0o750); err != nil {
		return "", fmt.Errorf("failed to create ssot cache directory: %w", err)
	}
	return cacheDir, nil
}

func openOrCloneRepo(repo, ref, targetPath string) (*git.Repository, bool, error) {
	// Attempt to open existing clone
	if repository, err := git.PlainOpen(targetPath); err == nil {
		if err := fetchLatest(repository); err != nil {
			if errors.Is(err, transport.ErrAuthenticationRequired) {
				return nil, false, err
			}
			logger.Debug("ssot: cached repo fetch failed, recloning", logger.String("path", targetPath), logger.String("error", err.Error()))
			_ = os.RemoveAll(targetPath)
		} else {
			return repository, true, nil
		}
	}

	// Clean target path (if any) before cloning
	_ = os.RemoveAll(targetPath)

	cloneURL, err := buildCloneURL(repo)
	if err != nil {
		return nil, false, err
	}

	logger.Info(fmt.Sprintf("Cloning SSOT repo %s (%s) into %s", repo, ref, targetPath))
	repository, err := git.PlainClone(targetPath, false, &git.CloneOptions{
		URL:          cloneURL,
		Progress:     nil,
		Tags:         git.AllTags,
		SingleBranch: false,
	})
	if err != nil {
		return nil, false, fmt.Errorf("failed to clone %s: %w", cloneURL, err)
	}

	return repository, false, nil
}

func fetchLatest(repository *git.Repository) error {
	err := repository.Fetch(&git.FetchOptions{
		RemoteName: "origin",
		Tags:       git.AllTags,
		Force:      true,
	})
	if err == nil || errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil
	}
	return err
}

func buildCloneURL(repo string) (string, error) {
	trimmed := strings.TrimSpace(repo)
	if strings.HasPrefix(trimmed, "http://") ||
		strings.HasPrefix(trimmed, "https://") ||
		strings.HasPrefix(trimmed, "ssh://") ||
		strings.HasPrefix(trimmed, "file://") {
		return trimmed, nil
	}

	if strings.Contains(trimmed, "://") {
		return "", fmt.Errorf("unsupported repo URL scheme: %s", trimmed)
	}

	trimmed = strings.TrimSuffix(trimmed, ".git")
	return fmt.Sprintf("https://github.com/%s.git", trimmed), nil
}

func hashRepoRef(repo, ref string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(repo) + ":" + ref))
	return hex.EncodeToString(sum[:])[:32]
}

func resolveRefHash(repository *git.Repository, ref string) (plumbing.Hash, error) {
	// Try to resolve using go-git's revision parser first
	if hash, err := repository.ResolveRevision(plumbing.Revision(ref)); err == nil {
		return *hash, nil
	}

	// Try refs/heads, refs/tags, refs/remotes
	candidates := []plumbing.ReferenceName{
		plumbing.ReferenceName(ref),
		plumbing.NewBranchReferenceName(ref),
		plumbing.NewRemoteReferenceName("origin", ref),
		plumbing.NewTagReferenceName(ref),
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if reference, err := repository.Reference(candidate, true); err == nil {
			return reference.Hash(), nil
		}
	}

	// Treat as commit hash (40 hex chars)
	if len(ref) == 40 && isHex(ref) {
		return plumbing.NewHash(ref), nil
	}

	return plumbing.ZeroHash, fmt.Errorf("ref %s not found", ref)
}

func checkoutHash(repository *git.Repository, hash plumbing.Hash) error {
	worktree, err := repository.Worktree()
	if err != nil {
		return err
	}
	return worktree.Checkout(&git.CheckoutOptions{
		Hash:  hash,
		Force: true,
	})
}

func isHex(s string) bool {
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}
