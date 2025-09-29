package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/fulmenhq/goneat/pkg/config"
	"github.com/fulmenhq/goneat/pkg/logger"
)

const (
	// DirectoryName is the subdirectory inside goneat home where server metadata lives.
	DirectoryName = "servers"

	// PortMin and PortMax define the ephemeral port range guardian servers should use.
	PortMin = 49152
	PortMax = 65535

	metadataExt = ".json"
)

// Info represents a managed goneat auxiliary server instance.
type Info struct {
	Name      string            `json:"name"`
	Port      int               `json:"port"`
	PID       int               `json:"pid"`
	Version   string            `json:"version"`
	StartedAt time.Time         `json:"started_at"`
	Endpoint  string            `json:"endpoint,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// HelloResponse is returned by managed servers from the /hello endpoint.
type HelloResponse struct {
	Name      string    `json:"name"`
	Version   string    `json:"version"`
	StartedAt time.Time `json:"started_at"`
}

// Save writes server metadata to disk, overwriting any existing record for the name.
func Save(info Info) error {
	if info.Name == "" {
		return errors.New("server info missing name")
	}
	if info.Port <= 0 {
		return fmt.Errorf("server %s missing port", info.Name)
	}

	path, err := metadataPath(info.Name)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal server metadata: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write server metadata %s: %w", info.Name, err)
	}
	return nil
}

// Remove deletes the metadata record for a server name.
func Remove(name string) error {
	if name == "" {
		return errors.New("server name required for removal")
	}
	path, err := metadataPath(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove server metadata %s: %w", name, err)
	}
	return nil
}

// Load retrieves the metadata record for a server name.
func Load(name string) (*Info, error) {
	if name == "" {
		return nil, errors.New("server name required")
	}
	path, err := metadataPath(name)
	if err != nil {
		return nil, err
	}

	raw, err := os.ReadFile(path) // #nosec G304 -- path constructed from goneat home directory
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read server metadata %s: %w", name, err)
	}

	var info Info
	if err := json.Unmarshal(raw, &info); err != nil {
		return nil, fmt.Errorf("parse server metadata %s: %w", name, err)
	}
	return &info, nil
}

// List returns all registered server metadata records sorted by name.
func List() ([]Info, error) {
	dir, err := ensureDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read server directory: %w", err)
	}

	infos := make([]Info, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != metadataExt {
			continue
		}
		name := entry.Name()[:len(entry.Name())-len(metadataExt)]
		info, err := Load(name)
		if err != nil {
			// Skip invalid metadata but continue processing.
			continue
		}
		if info != nil {
			infos = append(infos, *info)
		}
	}

	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos, nil
}

// ProbeHello calls the /hello endpoint for the provided server info.
func ProbeHello(info Info, client *http.Client) (*HelloResponse, error) {
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Second}
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/hello", info.Port)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Error("Failed to close response body", logger.Err(err))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var hello HelloResponse
	if err := json.NewDecoder(resp.Body).Decode(&hello); err != nil {
		return nil, fmt.Errorf("decode hello response: %w", err)
	}
	return &hello, nil
}

// IsPortAvailable returns true when the port is free on localhost.
func IsPortAvailable(port int) bool {
	if port <= 0 {
		return false
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func metadataPath(name string) (string, error) {
	dir, err := ensureDir()
	if err != nil {
		return "", err
	}
	filename := fmt.Sprintf("%s%s", name, metadataExt)
	return filepath.Join(dir, filename), nil
}

func ensureDir() (string, error) {
	home, err := config.EnsureGoneatHome()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, DirectoryName)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("failed to create server metadata directory: %w", err)
	}
	return dir, nil
}
