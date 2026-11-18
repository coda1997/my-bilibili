package fixture

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image/png"
	"io"
	"my-bilibili/internal/app"
	"os"
	"path/filepath"
	"testing"
	"time"

	"my-bilibili/internal/catalog"
	"my-bilibili/internal/playback"
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
	video, err := s.Detail(context.Background())
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
		r, err := s.Resolve(context.Background(), app.ResolveRequest{Part: part.ID})
		in := r.Input
		if !r.ExpiresAt.IsZero() {
			t.Fatal("bundled fixture input must not expire")
		}
		if err != nil {
			t.Fatal(err)
		}
		if err := in.Validate(); err != nil {
			t.Fatal(err)
		}
		if !filepath.IsAbs(in.FilePath) || filepath.Dir(in.FilePath) != dir || r.Input.URL != "" {
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
	again, _ := s.Detail(context.Background())
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

func TestFixtureQualityIsExactOwnedAndCannotClaimServiceChoices(t *testing.T) {
	dir := copyAssets(t)
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := app.Quality{ID: "fixture", Label: "Fixture", Width: 640, Height: 360}
	for _, requested := range []app.QualityID{"", "fixture"} {
		r, err := s.Resolve(context.Background(), app.ResolveRequest{Part: "part-1", Quality: requested})
		if err != nil || r.Quality != want || len(r.Qualities) != 1 || r.Qualities[0] != want || r.RefreshOnRetry || !r.ExpiresAt.IsZero() {
			t.Fatal("fixture did not return its one actual local choice", r.Quality, len(r.Qualities), err)
		}
		r.Qualities[0].Label = "caller mutation"
	}
	// An impossible quality must fail before the resource is read.
	if err := os.Remove(filepath.Join(dir, "part-1.mp4")); err != nil {
		t.Fatal(err)
	}
	for _, requested := range []app.QualityID{"64", "unknown", "FIXTURE"} {
		r, err := s.Resolve(context.Background(), app.ResolveRequest{Part: "part-1", Quality: requested})
		if err != catalog.QualityUnavailable || r.Input.FilePath != "" || r.Quality != (app.Quality{}) || len(r.Qualities) != 0 {
			t.Fatal("unknown quality read the asset or silently chose a default", r.Quality, err)
		}
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
	r, err := s.Resolve(context.Background(), app.ResolveRequest{Part: "part-1"})
	in := r.Input
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
			if err != playback.Media {
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
			if err != playback.Media {
				t.Fatalf("%v", err)
			}
		})
		t.Run("size/"+name, func(t *testing.T) {
			dir := copyAssets(t)
			if err := os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := New(dir)
			if err != playback.Media {
				t.Fatalf("%v", err)
			}
		})
	}
	if _, err := New(""); err != playback.Invalid {
		t.Fatal(err)
	}
	if _, err := New(filepath.Join(t.TempDir(), "missing-private-directory")); err != playback.Media {
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
	if _, err := New(dir); err != playback.Media {
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
		if r, err := s.Resolve(context.Background(), app.ResolveRequest{Part: catalog.PartID(id)}); err != playback.Invalid || !r.ExpiresAt.IsZero() || r.Input.FilePath != "" || r.Input.URL != "" || r.Input.AudioURL != "" || len(r.Input.Headers) != 0 {
			t.Fatalf("unknown ID accepted: %v", err)
		}
	}
	path := filepath.Join(dir, "part-1.mp4")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Resolve(context.Background(), app.ResolveRequest{Part: "part-1"}); err != playback.Media {
		t.Fatal("missing resource was not rechecked", err)
	}
	if _, err := s.Resolve(context.Background(), app.ResolveRequest{Part: "part-2"}); err != nil {
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
			if _, err := New(dir); err != playback.Media {
				t.Fatalf("symlinked resource accepted: %v", err)
			}
		})
	}
}

func TestSourceRejectsCanceledAndNilContexts(t *testing.T) {
	dir := copyAssets(t)
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Detail(nil); err != playback.Invalid {
		t.Fatal("nil detail context", err)
	}
	if _, err := s.Resolve(nil, app.ResolveRequest{Part: "part-1"}); err != playback.Invalid {
		t.Fatal("nil resolve context", err)
	}
	// A canceled call must stop before touching a now-missing media file.
	if err := os.Remove(filepath.Join(dir, "part-1.mp4")); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, stop := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer stop()
	for _, ctx := range []context.Context{canceled, expired} {
		if video, err := s.Detail(ctx); err != ctx.Err() || len(video.Parts) != 0 {
			t.Fatal("canceled detail published metadata", err)
		}
		if result, err := s.Resolve(ctx, app.ResolveRequest{Part: "part-1"}); err != ctx.Err() || result.Input.FilePath != "" || !result.ExpiresAt.IsZero() {
			t.Fatal("canceled resolve touched or published media", err)
		}
	}
}

type trackedReader struct {
	reader                io.Reader
	reads, bytes, largest int
	cancelAt, closed      int
	cancel                context.CancelFunc
	readErr, closeErr     error
}

func (r *trackedReader) Read(buffer []byte) (int, error) {
	r.reads++
	r.largest = max(r.largest, len(buffer))
	if r.reads == r.cancelAt {
		r.cancel()
	}
	if r.readErr != nil {
		return 0, r.readErr
	}
	n, err := r.reader.Read(buffer)
	r.bytes += n
	return n, err
}

func (r *trackedReader) Close() error {
	r.closed++
	return r.closeErr
}

func contentAsset(data []byte) asset {
	sum := sha256.Sum256(data)
	return asset{Bytes: int64(len(data)), SHA256: hex.EncodeToString(sum[:])}
}

func TestVerificationCancelsAroundBoundedReadsAndClosesOwnership(t *testing.T) {
	for _, test := range []struct {
		name     string
		size     int
		cancelAt int
	}{
		{"before-read", 96 * 1024, 0},
		{"last-read", 1, 1},
		{"between-reads", 96 * 1024, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := bytes.Repeat([]byte{0x5a}, test.size)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			reader := &trackedReader{reader: bytes.NewReader(data), cancelAt: test.cancelAt, cancel: cancel}
			if test.cancelAt == 0 {
				cancel()
			}
			if err := verifyContents(ctx, reader, contentAsset(data)); err != context.Canceled {
				t.Fatal("cancellation was lost during file verification", err)
			}
			if reader.closed != 1 || reader.reads != test.cancelAt || reader.largest > 32*1024 {
				t.Fatalf("verification ownership/bound: closed=%d reads=%d largest=%d", reader.closed, reader.reads, reader.largest)
			}
		})
	}
}

func TestVerificationPreservesIntegrityAndSanitizesIOFailures(t *testing.T) {
	data := []byte("fixture bytes")
	for _, test := range []struct {
		name     string
		data     []byte
		readErr  error
		closeErr error
		want     error
	}{
		{"valid", data, nil, nil, nil},
		{"short", data[:len(data)-1], nil, nil, playback.Media},
		{"long", bytes.Repeat(data, 100), nil, nil, playback.Media},
		{"changed", bytes.Repeat([]byte{'x'}, len(data)), nil, nil, playback.Media},
		{"read-error", data, errors.New("private file path"), nil, playback.Media},
		{"close-error", data, nil, errors.New("private file path"), playback.Media},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader := &trackedReader{reader: bytes.NewReader(test.data), readErr: test.readErr, closeErr: test.closeErr}
			if err := verifyContents(context.Background(), reader, contentAsset(data)); err != test.want {
				t.Fatal("verification result", err)
			}
			if reader.closed != 1 || reader.bytes > len(data)+1 {
				t.Fatal("file ownership or total read bound", reader.closed, reader.bytes)
			}
		})
	}
}
