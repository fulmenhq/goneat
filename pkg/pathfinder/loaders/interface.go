package loaders

import (
	"fmt"
	"io"

	"github.com/fulmenhq/goneat/pkg/pathfinder"
)

// SourceLoader implementations provide access to different file sources
// This file defines common types and utilities for all loaders

// FileInfo wraps os.FileInfo with additional pathfinder-specific information
type FileInfo struct {
	Path    string
	Name    string
	Size    int64
	Mode    pathfinder.FileMode
	ModTime int64 // Unix timestamp
	IsDir   bool
}

// NewFileInfo creates a FileInfo from os.FileInfo
func NewFileInfo(path string, info pathfinder.FileInfo) *FileInfo {
	return &FileInfo{
		Path:    path,
		Name:    info.Name(),
		Size:    info.Size(),
		Mode:    pathfinder.FileMode(info.Mode()),
		ModTime: info.ModTime().Unix(),
		IsDir:   info.IsDir(),
	}
}

// ReadSeekCloser combines io.Reader, io.Seeker, and io.Closer
type ReadSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

// LoaderError represents loader-specific errors
type LoaderError struct {
	Type    string
	Message string
	Path    string
	Cause   error
}

func (e *LoaderError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (path: %s): %v", e.Type, e.Message, e.Path, e.Cause)
	}
	return fmt.Sprintf("%s: %s (path: %s)", e.Type, e.Message, e.Path)
}

// Common loader configuration
type CommonConfig struct {
	AuditTrail     bool                   `json:"audit_trail"`
	MaxFileSize    int64                  `json:"max_file_size,omitempty"`
	TimeoutSeconds int                    `json:"timeout_seconds,omitempty"`
	CustomConfig   map[string]interface{} `json:"custom,omitempty"`
}
