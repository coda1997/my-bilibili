// Package fixture supplies the original, offline M2 preview resources.
package fixture

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"

	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/playback"
)

// The binary pins the manifest as well as each asset. Replacing both an asset
// and its adjacent manifest must not turn an unverified resource into a fixture.
//
//go:embed assets/manifest.json
var expectedManifest []byte

type asset struct {
	Name   string `json:"name"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type source struct {
	dir   string
	video catalog.Video
	parts map[catalog.PartID]asset
}

// New validates the fixed resource set beneath resourceDir. Composition chooses
// the directory relative to the executable/bundle; this adapter captures its
// absolute path so later process working-directory changes cannot redirect it.
func New(resourceDir string) (app.Source, error) {
	if resourceDir == "" {
		return nil, playback.Invalid
	}
	dir, err := filepath.Abs(resourceDir)
	if err != nil {
		return nil, playback.Media
	}
	manifestPath := filepath.Join(dir, "manifest.json")
	info, err := os.Lstat(manifestPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() != int64(len(expectedManifest)) {
		return nil, playback.Media
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil || !bytes.Equal(data, expectedManifest) {
		return nil, playback.Media
	}
	var manifest struct {
		Version int     `json:"version"`
		Files   []asset `json:"files"`
	}
	if json.Unmarshal(data, &manifest) != nil {
		return nil, playback.Media
	}
	s := &source{dir: dir, parts: make(map[catalog.PartID]asset)}
	for _, file := range manifest.Files {
		if err := verify(context.Background(), dir, file); err != nil {
			return nil, err
		}
		switch file.Name {
		case "part-1.mp4":
			s.parts["part-1"] = file
		case "part-2.mp4":
			s.parts["part-2"] = file
		}
	}
	data, err = os.ReadFile(filepath.Join(dir, "metadata.json"))
	if err != nil {
		return nil, playback.Media
	}
	var metadata struct {
		ID          catalog.VideoID `json:"id"`
		Title       string          `json:"title"`
		Creator     string          `json:"creator"`
		Description string          `json:"description"`
		CoverID     string          `json:"cover_id"`
		Parts       []struct {
			ID         catalog.PartID `json:"id"`
			Title      string         `json:"title"`
			DurationMS int64          `json:"duration_ms"`
		} `json:"parts"`
	}
	if json.Unmarshal(data, &metadata) != nil {
		return nil, playback.Media
	}
	s.video = catalog.Video{ID: metadata.ID, Title: metadata.Title, Creator: metadata.Creator, Description: metadata.Description, CoverID: metadata.CoverID}
	for _, part := range metadata.Parts {
		s.video.Parts = append(s.video.Parts, catalog.Part{ID: part.ID, Title: part.Title, Duration: time.Duration(part.DurationMS) * time.Millisecond})
	}
	return s, nil
}

func verify(ctx context.Context, dir string, file asset) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path := filepath.Join(dir, file.Name)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() != file.Bytes {
		return playback.Media
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return playback.Media
	}
	return verifyContents(ctx, f, file)
}

// verifyContents owns and closes the reader on every path. Cancellation is
// checked around bounded reads; a stalled OS file syscall is not interruptible
// through this context. The extra byte detects growth since the size check.
func verifyContents(ctx context.Context, f io.ReadCloser, file asset) (err error) {
	defer func() {
		if closeErr := f.Close(); err == nil && closeErr != nil {
			err = playback.Media
		}
	}()
	h := sha256.New()
	r := io.LimitReader(f, file.Bytes+1)
	buffer := make([]byte, 32*1024)
	var size int64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, readErr := r.Read(buffer)
		if err := ctx.Err(); err != nil {
			return err
		}
		size += int64(n)
		_, _ = h.Write(buffer[:n])
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return playback.Media
		}
	}
	if size != file.Bytes || hex.EncodeToString(h.Sum(nil)) != file.SHA256 {
		return playback.Media
	}
	return nil
}

func (s *source) Detail(ctx context.Context) (catalog.Video, error) {
	if ctx == nil {
		return catalog.Video{}, playback.Invalid
	}
	if err := ctx.Err(); err != nil {
		return catalog.Video{}, err
	}
	video := s.video
	video.Parts = append([]catalog.Part(nil), video.Parts...)
	return video, nil
}

func (s *source) Resolve(ctx context.Context, request app.ResolveRequest) (app.Resolution, error) {
	if ctx == nil {
		return app.Resolution{}, playback.Invalid
	}
	if err := ctx.Err(); err != nil {
		return app.Resolution{}, err
	}
	file, ok := s.parts[request.Part]
	if !ok {
		return app.Resolution{}, playback.Invalid
	}
	quality := app.Quality{ID: "fixture", Label: "Fixture", Width: 640, Height: 360}
	if request.Quality != "" && request.Quality != quality.ID {
		return app.Resolution{}, catalog.QualityUnavailable
	}
	if err := verify(ctx, s.dir, file); err != nil {
		return app.Resolution{}, err
	}
	in := playback.Input{FilePath: filepath.Join(s.dir, file.Name)}
	if err := in.Validate(); err != nil {
		return app.Resolution{}, playback.Invalid
	}
	if err := ctx.Err(); err != nil {
		return app.Resolution{}, err
	}
	// Bundled local files do not expire.
	return app.Resolution{Input: in, Quality: quality, Qualities: []app.Quality{quality}}, nil
}
