package vision

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/disintegration/imaging"
)

func TestPreprocessImageFile_scalesAndLimitsPayload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.png")
	img := imaging.New(3000, 2000, color.White)
	if err := imaging.Save(img, path); err != nil {
		t.Fatal(err)
	}

	out, meta, err := PreprocessImageFile(path, PreprocessOptions{
		MaxImageBytes:            10 * 1024 * 1024,
		MaxDimension:             1024,
		JPEGQuality:              85,
		MaxPayloadBytes:          600 * 1024,
		SkipPreprocessBelowBytes: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Bytes) == 0 {
		t.Fatal("empty output")
	}
	if meta.PreprocessMode != "jpeg" {
		t.Fatalf("mode: %s", meta.PreprocessMode)
	}
	if meta.OutputWidth > 1024 || meta.OutputHeight > 1024 {
		t.Fatalf("expected fit within 1024, got %dx%d", meta.OutputWidth, meta.OutputHeight)
	}
	if int64(len(out.Bytes)) > 600*1024 {
		t.Fatalf("payload %d exceeds max", len(out.Bytes))
	}
}

func TestPreprocessImageFile_passthroughSmallPNG(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "small.png")
	if err := imaging.Save(imaging.New(400, 300, color.White), path); err != nil {
		t.Fatal(err)
	}

	out, meta, err := PreprocessImageFile(path, PreprocessOptions{
		MaxImageBytes:            5 * 1024 * 1024,
		MaxDimension:             2048,
		MaxPayloadBytes:          512 * 1024,
		SkipPreprocessBelowBytes: 2 * 1024 * 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	if meta.PreprocessMode != "passthrough" {
		t.Fatalf("expected passthrough, got %s", meta.PreprocessMode)
	}
	if out.MIMEType != "image/png" {
		t.Fatalf("mime: %s", out.MIMEType)
	}
	if meta.OutputWidth != 400 || meta.OutputHeight != 300 {
		t.Fatalf("dims: %dx%d", meta.OutputWidth, meta.OutputHeight)
	}
}

func TestPreprocessImageFile_passthroughDisabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "small.png")
	if err := imaging.Save(imaging.New(100, 100, color.White), path); err != nil {
		t.Fatal(err)
	}

	_, meta, err := PreprocessImageFile(path, PreprocessOptions{
		MaxDimension:             2048,
		MaxPayloadBytes:          512 * 1024,
		SkipPreprocessBelowBytes: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if meta.PreprocessMode != "jpeg" {
		t.Fatalf("expected jpeg compress, got %s", meta.PreprocessMode)
	}
}

func TestPreprocessImageFile_rejectsOversizeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tiny.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	f.Close()

	_, _, err = PreprocessImageFile(path, PreprocessOptions{MaxImageBytes: 1})
	if err == nil {
		t.Fatal("expected error when file exceeds max_image_bytes")
	}
}
