package fixture

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"my-bilibili/experiments/playback/contract"
	"my-bilibili/internal/catalog"
)

func copyAssets(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	entries, err := os.ReadDir("assets")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join("assets", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, entry.Name()), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestOriginalResourcesAndCatalog(t *testing.T) {
	dir := copyAssets(t)
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	video, err := s.Detail()
	if err != nil {
		t.Fatal(err)
	}
	if video.ID != "preview-video" || video.CoverID != "preview-cover" || len(video.Parts) != 2 || video.Title == "" || video.Creator == "" || video.Description == "" {
		t.Fatal(video)
	}
	if video.Parts[0].ID != "part-1" || video.Parts[1].ID != "part-2" || video.Parts[0].Title == video.Parts[1].Title {
		t.Fatal(video.Parts)
	}
	paths := make(map[string]bool)
	hashes := make(map[[32]byte]bool)
	for _, part := range video.Parts {
		if part.Duration != 30*time.Second {
			t.Fatal(part)
		}
		in, err := s.Resolve(part.ID)
		if err != nil {
			t.Fatal(err)
		}
		if err := in.Validate(); err != nil {
			t.Fatal(err)
		}
		if !filepath.IsAbs(in.FilePath) || filepath.Dir(in.FilePath) != dir || in.URL != "" || in.AudioURL != "" || len(in.Headers) != 0 {
			t.Fatal("incorrect resource mapping")
		}
		data, err := os.ReadFile(in.FilePath)
		if err != nil {
			t.Fatal(err)
		}
		paths[in.FilePath], hashes[sha256.Sum256(data)] = true, true
	}
	if len(paths) != 2 || len(hashes) != 2 {
		t.Fatal("parts do not resolve to distinct media")
	}
	video.Parts[0].Title = "mutated"
	again, _ := s.Detail()
	if again.Parts[0].Title == "mutated" {
		t.Fatal("catalog slice alias")
	}
	f, err := os.Open(filepath.Join(dir, "cover.png"))
	if err != nil {
		t.Fatal(err)
	}
	config, err := png.DecodeConfig(f)
	closeErr := f.Close()
	if err != nil || closeErr != nil || config.Width != 640 || config.Height != 360 {
		t.Fatalf("cover=%+v err=%v close=%v", config, err, closeErr)
	}
}

func TestRelativeDirectoryCapturedAtConstruction(t *testing.T) {
	dir := copyAssets(t)
	t.Chdir(dir)
	s, err := New(".")
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	in, err := s.Resolve("part-1")
	if err != nil {
		t.Fatal(err)
	}
	if in.FilePath != filepath.Join(dir, "part-1.mp4") {
		t.Fatal("resolution depends on later cwd")
	}
}

func TestMissingMalformedAndTamperedResources(t *testing.T) {
	for _, name := range []string{"manifest.json", "metadata.json", "cover.png", "part-1.mp4", "part-2.mp4"} {
		t.Run("missing/"+name, func(t *testing.T) {
			dir := copyAssets(t)
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				t.Fatal(err)
			}
			_, err := New(dir)
			if err != contract.Media {
				t.Fatalf("%v", err)
			}
		})
		t.Run("changed/"+name, func(t *testing.T) {
			dir := copyAssets(t)
			path := filepath.Join(dir, name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			data[len(data)/2] ^= 0x40
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			_, err = New(dir)
			if err != contract.Media {
				t.Fatalf("%v", err)
			}
		})
		t.Run("size/"+name, func(t *testing.T) {
			dir := copyAssets(t)
			if err := os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := New(dir)
			if err != contract.Media {
				t.Fatalf("%v", err)
			}
		})
	}
	if _, err := New(""); err != contract.Invalid {
		t.Fatal(err)
	}
	if _, err := New(filepath.Join(t.TempDir(), "missing-private-directory")); err != contract.Media {
		t.Fatal(err)
	}
}

func TestAdjacentManifestCannotAuthorizeChangedMedia(t *testing.T) {
	dir := copyAssets(t)
	path := filepath.Join(dir, "part-1.mp4")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)/2] ^= 1
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Version int     `json:"version"`
		Files   []asset `json:"files"`
	}
	if err := json.Unmarshal(expectedManifest, &manifest); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	manifest.Files[0].SHA256 = hex.EncodeToString(sum[:])
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded, '\n')
	if len(encoded) != len(expectedManifest) {
		t.Fatal("probe must preserve manifest size")
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), encoded, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(dir); err != contract.Media {
		t.Fatal("manifest replacement authorized changed asset", err)
	}
}

func TestResolveRechecksFileAndRejectsUnknownIDs(t *testing.T) {
	dir := copyAssets(t)
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"", "../part-1.mp4", "part-3", "PART-1"} {
		if in, err := s.Resolve(catalog.PartID(id)); err != contract.Invalid || in.FilePath != "" || in.URL != "" || in.AudioURL != "" || len(in.Headers) != 0 {
			t.Fatalf("unknown ID accepted: %v", err)
		}
	}
	path := filepath.Join(dir, "part-1.mp4")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Resolve("part-1"); err != contract.Media {
		t.Fatal("missing resource was not rechecked", err)
	}
	if _, err := s.Resolve("part-2"); err != nil {
		t.Fatal("unrelated part affected", err)
	}
}

func TestResourceSymlinksAreNotRegularAssets(t *testing.T) {
	for _, name := range []string{"manifest.json", "part-1.mp4"} {
		t.Run(name, func(t *testing.T) {
			dir := copyAssets(t)
			path := filepath.Join(dir, name)
			target := filepath.Join(t.TempDir(), name)
			if err := os.Rename(path, target); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, path); err != nil {
				t.Skipf("symlink creation unavailable: %v", err)
			}
			if _, err := New(dir); err != contract.Media {
				t.Fatalf("symlinked resource accepted: %v", err)
			}
		})
	}
}
