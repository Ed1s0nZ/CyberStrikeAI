package workflowpackage

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
)

func TestInspectArchiveAcceptsSingleVerifiedWorkflow(t *testing.T) {
	pkg, meta, err := Export(testDocument())
	if err != nil {
		t.Fatal(err)
	}
	result, err := InspectArchive(context.Background(), pkg, func(context.Context, string) error { return nil })
	if err != nil {
		t.Fatalf("InspectArchive: %v", err)
	}
	if result.PackageHash != meta.PackageHash || result.Document.ID != "web-src-hunting" {
		t.Fatalf("unexpected inspection: %#v", result)
	}
	if result.NodeCount != 2 || result.EdgeCount != 1 {
		t.Fatalf("counts = %d/%d, want 2/1", result.NodeCount, result.EdgeCount)
	}
}

func TestInspectArchiveRejectsUnsafeArchiveShapes(t *testing.T) {
	valid, _, err := Export(testDocument())
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		archive []byte
	}{
		{name: "duplicate entry", archive: appendZipEntry(t, valid, "manifest.json", []byte(`{}`), 0)},
		{name: "path traversal", archive: appendZipEntry(t, valid, "../payload.json", []byte(`{}`), 0)},
		{name: "symlink", archive: appendZipEntry(t, valid, "workflows/link.json", []byte("target"), 0o120777)},
		{name: "undeclared file", archive: appendZipEntry(t, valid, "notes.txt", []byte("not allowed"), 0)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := InspectArchive(context.Background(), tc.archive, func(context.Context, string) error { return nil })
			if ErrorCode(err) != "WFPKG_INVALID_ARCHIVE" {
				t.Fatalf("code = %q, err = %v", ErrorCode(err), err)
			}
		})
	}
}

func TestInspectArchiveRejectsChecksumMismatchAndInvalidWorkflow(t *testing.T) {
	pkg, _, err := Export(testDocument())
	if err != nil {
		t.Fatal(err)
	}
	badChecksum := replaceZipEntry(t, pkg, "checksums.sha256", []byte("00  manifest.json\n"), 0)
	if _, err := InspectArchive(context.Background(), badChecksum, func(context.Context, string) error { return nil }); ErrorCode(err) != "WFPKG_CHECKSUM_MISMATCH" {
		t.Fatalf("checksum code = %q, err = %v", ErrorCode(err), err)
	}
	if _, err := InspectArchive(context.Background(), pkg, func(context.Context, string) error { return errors.New("invalid graph") }); ErrorCode(err) != "WFPKG_WORKFLOW_INVALID" {
		t.Fatalf("graph code = %q, err = %v", ErrorCode(err), err)
	}
}

func appendZipEntry(t *testing.T, archive []byte, name string, data []byte, mode os.FileMode) []byte {
	t.Helper()
	return rewriteZip(t, archive, func(zw *zip.Writer) error {
		h := &zip.FileHeader{Name: name, Method: zip.Store}
		h.SetMode(mode)
		w, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	})
}

func replaceZipEntry(t *testing.T, archive []byte, name string, data []byte, mode os.FileMode) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, f := range zr.File {
		if f.Name == name {
			h := &zip.FileHeader{Name: name, Method: zip.Store}
			h.SetMode(mode)
			w, err := zw.CreateHeader(h)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write(data); err != nil {
				t.Fatal(err)
			}
			continue
		}
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		h := &zip.FileHeader{Name: f.Name, Method: zip.Store}
		w, err := zw.CreateHeader(h)
		if err == nil {
			_, err = io.Copy(w, r)
		}
		_ = r.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func rewriteZip(t *testing.T, archive []byte, appendEntry func(*zip.Writer) error) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, f := range zr.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		h := &zip.FileHeader{Name: f.Name, Method: zip.Store}
		w, err := zw.CreateHeader(h)
		if err == nil {
			_, err = io.Copy(w, r)
		}
		_ = r.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := appendEntry(zw); err != nil {
		t.Fatal(fmt.Errorf("append entry: %w", err))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
