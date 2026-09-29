package command

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestListWallpaperFiles checks the library scan: images in subfolders are
// found, thumbs/ is skipped, unknown extensions are ignored, and videos are
// included only when the display allows them.
func TestListWallpaperFiles(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{
		"a.jpg", "b.PNG", "clip.mp4", "notes.txt",
		"sub/c.webp", "sub/deeper/clip2.webm",
		"thumbs/a.jpg", "thumbs/clip.jpg",
	} {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rel := func(files []string) []string {
		out := make([]string, len(files))
		for i, f := range files {
			out[i], _ = filepath.Rel(root, f)
		}
		slices.Sort(out)
		return out
	}

	images := rel(listWallpaperFiles(Display{Seed: 1}, root))
	if want := []string{"a.jpg", "b.PNG", "sub/c.webp"}; !slices.Equal(images, want) {
		t.Errorf("images only: got %v, want %v", images, want)
	}

	all := rel(listWallpaperFiles(Display{Seed: 1, Video: true}, root))
	if want := []string{"a.jpg", "b.PNG", "clip.mp4", "sub/c.webp", "sub/deeper/clip2.webm"}; !slices.Equal(all, want) {
		t.Errorf("with videos: got %v, want %v", all, want)
	}

	// Same seed → same order; the order is what the rotation relies on.
	if !slices.Equal(listWallpaperFiles(Display{Seed: 7}, root), listWallpaperFiles(Display{Seed: 7}, root)) {
		t.Error("same seed produced different orders")
	}

	if got := listWallpaperFiles(Display{}, ""); got != nil {
		t.Errorf("empty root: got %v, want nil", got)
	}
}
