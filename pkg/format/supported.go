package format

import (
	"path/filepath"
	"strings"
)

var supportedContentTypes = []string{
	"go",
	"yaml",
	"json",
	"markdown",
	"python",
	"javascript",
	"typescript",
}

var contentTypeByExtension = map[string]string{
	".go":       "go",
	".yaml":     "yaml",
	".yml":      "yaml",
	".json":     "json",
	".xml":      "xml",
	".md":       "markdown",
	".markdown": "markdown",
	".py":       "python",
	".pyi":      "python",
	".js":       "javascript",
	".jsx":      "javascript",
	".mjs":      "javascript",
	".cjs":      "javascript",
	".ts":       "typescript",
	".tsx":      "typescript",
	".mts":      "typescript",
	".cts":      "typescript",
}

// SupportedContentTypes returns the content types handled by the format
// command's primary formatter paths.
func SupportedContentTypes() []string {
	return append([]string(nil), supportedContentTypes...)
}

// SupportedContentTypesHelp returns the canonical comma-separated help text.
func SupportedContentTypesHelp() string {
	return strings.Join(supportedContentTypes, ", ")
}

// ContentTypeForExtension returns the canonical format content type for ext.
func ContentTypeForExtension(ext string) string {
	if contentType, ok := contentTypeByExtension[strings.ToLower(ext)]; ok {
		return contentType
	}
	return "unknown"
}

// ContentTypeForPath returns the canonical format content type for path.
func ContentTypeForPath(path string) string {
	return ContentTypeForExtension(filepath.Ext(path))
}
