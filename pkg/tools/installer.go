package tools

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/fulmenhq/goneat/pkg/config"
	"github.com/fulmenhq/goneat/pkg/logger"
)

const (
	downloadTimeout = 5 * time.Minute
)

type InstallOptions struct {
	Version  string
	FromFile string
	Force    bool
}

type InstallResult struct {
	BinaryPath string
	Version    string
	Verified   bool
}

func GetToolsDir() (string, error) {
	goneatHome, err := config.EnsureGoneatHome()
	if err != nil {
		return "", fmt.Errorf("failed to get goneat home: %w", err)
	}
	toolsDir := filepath.Join(goneatHome, "tools")
	if err := os.MkdirAll(toolsDir, 0o750); err != nil {
		return "", fmt.Errorf("failed to create tools directory: %w", err)
	}
	return toolsDir, nil
}

func GetCacheDir() (string, error) {
	toolsDir, err := GetToolsDir()
	if err != nil {
		return "", err
	}
	cacheDir := filepath.Join(toolsDir, "cache")
	if err := os.MkdirAll(cacheDir, 0o750); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}
	return cacheDir, nil
}

func GetBinDir() (string, error) {
	toolsDir, err := GetToolsDir()
	if err != nil {
		return "", err
	}
	binDir := filepath.Join(toolsDir, "bin")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		return "", fmt.Errorf("failed to create bin directory: %w", err)
	}
	return binDir, nil
}

func InstallArtifact(tool Tool, opts InstallOptions) (*InstallResult, error) {
	if tool.Artifacts == nil {
		return nil, fmt.Errorf("tool %s does not have artifacts manifest", tool.Name)
	}

	version := opts.Version
	if version == "" {
		version = tool.Artifacts.DefaultVersion
	}

	versionArtifacts, ok := tool.Artifacts.Versions[version]
	if !ok {
		return nil, fmt.Errorf("version %s not found in artifacts manifest for %s", version, tool.Name)
	}

	artifact, err := selectArtifactForPlatform(versionArtifacts)
	if err != nil {
		return nil, fmt.Errorf("failed to select artifact: %w", err)
	}

	binDir, err := GetBinDir()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(binDir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create bin directory: %w", err)
	}

	binaryName := tool.Name
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	installedPath := filepath.Join(binDir, fmt.Sprintf("%s@%s", tool.Name, version), binaryName)

	if !opts.Force {
		if _, err := os.Stat(installedPath); err == nil {
			logger.Debug("artifact already installed", logger.String("path", installedPath))
			return &InstallResult{
				BinaryPath: installedPath,
				Version:    version,
				Verified:   true,
			}, nil
		}
	}

	var artifactPath string
	if opts.FromFile != "" {
		artifactPath = opts.FromFile
		logger.Info("using pre-downloaded artifact", logger.String("path", artifactPath))
	} else {
		downloadedPath, err := downloadArtifact(tool.Name, version, artifact)
		if err != nil {
			return nil, fmt.Errorf("failed to download artifact: %w", err)
		}
		artifactPath = downloadedPath
	}

	if err := verifyChecksum(artifactPath, artifact.SHA256); err != nil {
		return nil, fmt.Errorf("checksum verification failed: %w", err)
	}

	logger.Info("checksum verified successfully", logger.String("sha256", artifact.SHA256[:16]+"..."))

	if err := extractArtifact(artifactPath, installedPath, tool.Name, artifact.ExtractPath); err != nil {
		return nil, fmt.Errorf("failed to extract artifact: %w", err)
	}

	if err := os.Chmod(installedPath, 0o755); err != nil {
		return nil, fmt.Errorf("failed to make binary executable: %w", err)
	}

	logger.Info("artifact installed successfully", logger.String("path", installedPath))

	return &InstallResult{
		BinaryPath: installedPath,
		Version:    version,
		Verified:   true,
	}, nil
}

func selectArtifactForPlatform(artifacts VersionArtifacts) (*Artifact, error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	platformKey := fmt.Sprintf("%s_%s", goos, goarch)

	switch platformKey {
	case "darwin_amd64":
		if artifacts.DarwinAMD64 != nil {
			return artifacts.DarwinAMD64, nil
		}
	case "darwin_arm64":
		if artifacts.DarwinARM64 != nil {
			return artifacts.DarwinARM64, nil
		}
	case "linux_amd64":
		if artifacts.LinuxAMD64 != nil {
			return artifacts.LinuxAMD64, nil
		}
	case "linux_arm64":
		if artifacts.LinuxARM64 != nil {
			return artifacts.LinuxARM64, nil
		}
	case "windows_amd64":
		if artifacts.WindowsAMD64 != nil {
			return artifacts.WindowsAMD64, nil
		}
	}

	return nil, fmt.Errorf("no artifact available for platform %s/%s", goos, goarch)
}

func downloadArtifact(toolName, version string, artifact *Artifact) (string, error) {
	cacheDir, err := GetCacheDir()
	if err != nil {
		return "", err
	}

	toolCacheDir := filepath.Join(cacheDir, toolName, version)
	if err := os.MkdirAll(toolCacheDir, 0o750); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	filename := filepath.Base(artifact.URL)
	downloadPath := filepath.Join(toolCacheDir, filename)

	if _, err := os.Stat(downloadPath); err == nil {
		logger.Debug("using cached artifact", logger.String("path", downloadPath))
		return downloadPath, nil
	}

	logger.Info("downloading artifact", logger.String("url", artifact.URL))

	client := &http.Client{
		Timeout: downloadTimeout,
	}

	resp, err := client.Get(artifact.URL)
	if err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status: %s", resp.Status)
	}

	tmpFile := downloadPath + ".tmp"
	out, err := os.Create(tmpFile)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		os.Remove(tmpFile)
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	if err := os.Rename(tmpFile, downloadPath); err != nil {
		os.Remove(tmpFile)
		return "", fmt.Errorf("failed to move file: %w", err)
	}

	logger.Info("artifact downloaded", logger.String("path", downloadPath))

	return downloadPath, nil
}

func verifyChecksum(filePath, expectedSHA256 string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("failed to compute hash: %w", err)
	}

	actualSHA256 := hex.EncodeToString(hash.Sum(nil))

	if actualSHA256 != expectedSHA256 {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedSHA256, actualSHA256)
	}

	return nil
}

func extractArtifact(archivePath, targetPath, toolName, extractPath string) error {
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0o750); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	if strings.HasSuffix(archivePath, ".tar.gz") || strings.HasSuffix(archivePath, ".tgz") {
		return extractTarGz(archivePath, targetPath, toolName, extractPath)
	} else if strings.HasSuffix(archivePath, ".zip") {
		return extractZip(archivePath, targetPath, toolName, extractPath)
	}

	return fmt.Errorf("unsupported archive format: %s", archivePath)
}

func extractTarGz(archivePath, targetPath, toolName, extractPath string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	binaryName := toolName
	if extractPath != "" {
		binaryName = extractPath
	}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar: %w", err)
		}

		if filepath.Base(header.Name) == binaryName || strings.HasSuffix(header.Name, "/"+binaryName) {
			out, err := os.Create(targetPath)
			if err != nil {
				return fmt.Errorf("failed to create output file: %w", err)
			}
			defer out.Close()

			if _, err := io.Copy(out, tr); err != nil {
				return fmt.Errorf("failed to extract file: %w", err)
			}

			return nil
		}
	}

	return fmt.Errorf("binary %s not found in archive", binaryName)
}

func extractZip(archivePath, targetPath, toolName, extractPath string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}
	defer r.Close()

	binaryName := toolName
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	if extractPath != "" {
		binaryName = extractPath
	}

	for _, f := range r.File {
		if filepath.Base(f.Name) == binaryName || strings.HasSuffix(f.Name, "/"+binaryName) {
			rc, err := f.Open()
			if err != nil {
				return fmt.Errorf("failed to open file in zip: %w", err)
			}
			defer rc.Close()

			out, err := os.Create(targetPath)
			if err != nil {
				return fmt.Errorf("failed to create output file: %w", err)
			}
			defer out.Close()

			if _, err := io.Copy(out, rc); err != nil {
				return fmt.Errorf("failed to extract file: %w", err)
			}

			return nil
		}
	}

	return fmt.Errorf("binary %s not found in archive", binaryName)
}

func FindToolBinary(toolName string) (string, error) {
	binDir, err := GetBinDir()
	if err != nil {
		logger.Debug("failed to get bin directory, falling back to PATH", logger.String("error", err.Error()))
	} else {
		entries, err := os.ReadDir(binDir)
		if err == nil {
			binaryName := toolName
			if runtime.GOOS == "windows" {
				binaryName += ".exe"
			}

			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), toolName+"@") {
					binaryPath := filepath.Join(binDir, entry.Name(), binaryName)
					if _, err := os.Stat(binaryPath); err == nil {
						logger.Debug("found tool in managed binaries", logger.String("path", binaryPath))
						return binaryPath, nil
					}
				}
			}
		}
	}

	pathBinary, err := exec.LookPath(toolName)
	if err != nil {
		return "", fmt.Errorf("tool %s not found in managed binaries or PATH: %w", toolName, err)
	}

	logger.Debug("found tool in PATH", logger.String("path", pathBinary))
	return pathBinary, nil
}
