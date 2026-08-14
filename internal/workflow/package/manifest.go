package workflowpackage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	PackageFormat = "cyberstrikeai.workflow-package"
	FormatVersion = "1.0"
)

// Document is the single non-executable workflow definition carried by a package.
// Version is the source instance revision and is never applied as a target version.
type Document struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Version     int       `json:"version"`
	GraphJSON   string    `json:"graph_json"`
	Enabled     bool      `json:"enabled"`
	UpdatedAt   time.Time `json:"-"`
}

type Manifest struct {
	PackageFormat string         `json:"package_format"`
	FormatVersion string         `json:"format_version"`
	PackageID     string         `json:"package_id"`
	CreatedAt     string         `json:"created_at"`
	Items         []ManifestItem `json:"items"`
}

type ManifestItem struct {
	Type           string `json:"type"`
	Path           string `json:"path"`
	SourceID       string `json:"source_id"`
	SourceRevision int    `json:"source_revision"`
	ContentHash    string `json:"content_hash"`
	GraphHash      string `json:"graph_hash"`
}

type ExportMetadata struct {
	PackageHash    string
	ContentHash    string
	GraphHash      string
	SourceRevision int
	FileName       string
}

func canonicalJSON(raw []byte) ([]byte, error) {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, fmt.Errorf("extra JSON values")
	}
	return json.Marshal(value)
}

func sha256Prefixed(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func validHash(value string) bool {
	if !strings.HasPrefix(value, "sha256:") || len(value) != len("sha256:")+64 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil && value == strings.ToLower(value)
}
