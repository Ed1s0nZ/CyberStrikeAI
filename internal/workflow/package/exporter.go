package workflowpackage

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"
)

// Export builds a deterministic, human-readable single-workflow package.
func Export(source Document) ([]byte, ExportMetadata, error) {
	source.ID = strings.TrimSpace(source.ID)
	source.Name = strings.TrimSpace(source.Name)
	if source.ID == "" || source.Name == "" || source.Version <= 0 || !safePackageWorkflowID(source.ID) {
		return nil, ExportMetadata{}, fmt.Errorf("workflow id, name and version are required")
	}
	if strings.TrimSpace(source.GraphJSON) == "" {
		return nil, ExportMetadata{}, fmt.Errorf("workflow graph_json is required")
	}
	contentHash, graphHash, payload, err := DocumentHashes(source)
	if err != nil {
		return nil, ExportMetadata{}, err
	}
	workflowPath := path.Join("workflows", source.ID+".json")
	createdAt := source.UpdatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Unix(0, 0).UTC()
	}
	manifest := Manifest{
		PackageFormat: PackageFormat,
		FormatVersion: FormatVersion,
		PackageID:     "pkg_" + strings.TrimPrefix(contentHash, "sha256:")[:16],
		CreatedAt:     createdAt.Format(time.RFC3339),
		Items: []ManifestItem{{
			Type:           "workflow",
			Path:           workflowPath,
			SourceID:       source.ID,
			SourceRevision: source.Version,
			ContentHash:    contentHash,
			GraphHash:      graphHash,
		}},
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return nil, ExportMetadata{}, fmt.Errorf("marshal manifest: %w", err)
	}
	checksums := fmt.Sprintf("%s  manifest.json\n%s  %s\n", strings.TrimPrefix(sha256Prefixed(manifestBytes), "sha256:"), strings.TrimPrefix(contentHash, "sha256:"), workflowPath)

	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, entry := range []struct {
		name string
		data []byte
	}{
		{name: "checksums.sha256", data: []byte(checksums)},
		{name: "manifest.json", data: manifestBytes},
		{name: workflowPath, data: payload},
	} {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Store}
		header.SetModTime(time.Unix(0, 0).UTC())
		writer, err := zw.CreateHeader(header)
		if err != nil {
			return nil, ExportMetadata{}, fmt.Errorf("write %s: %w", entry.name, err)
		}
		if _, err := writer.Write(entry.data); err != nil {
			return nil, ExportMetadata{}, fmt.Errorf("write %s: %w", entry.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, ExportMetadata{}, fmt.Errorf("close package: %w", err)
	}
	pkg := out.Bytes()
	return pkg, ExportMetadata{
		PackageHash:    sha256Prefixed(pkg),
		ContentHash:    contentHash,
		GraphHash:      graphHash,
		SourceRevision: source.Version,
		FileName:       source.ID + ".csapkg.zip",
	}, nil
}

// DocumentHashes returns the canonical package item and graph hashes together
// with the canonical item bytes used by export and inspection persistence.
func DocumentHashes(source Document) (string, string, []byte, error) {
	graph, err := canonicalJSON([]byte(source.GraphJSON))
	if err != nil {
		return "", "", nil, fmt.Errorf("canonicalize graph_json: %w", err)
	}
	source.GraphJSON = string(graph)
	payload, err := json.Marshal(source)
	if err != nil {
		return "", "", nil, fmt.Errorf("marshal workflow payload: %w", err)
	}
	return sha256Prefixed(payload), sha256Prefixed(graph), payload, nil
}
