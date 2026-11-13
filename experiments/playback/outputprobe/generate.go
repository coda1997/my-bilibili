package outputprobe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type GenerateOptions struct {
	Destination string
	FFmpeg      string
	FFprobe     string
	Timeout     time.Duration
}
type MediaIdentity struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}
type ServerManifest struct {
	Origin   string          `json:"origin"`
	Fixtures []MediaIdentity `json:"fixtures"`
}
type DecodedEvent struct {
	ID               int     `json:"id"`
	VideoPTS         float64 `json:"video_pts_seconds"`
	AudioPTS         float64 `json:"audio_pts_seconds"`
	Offset           float64 `json:"audio_minus_video_seconds"`
	VideoUncertainty float64 `json:"video_uncertainty_seconds"`
	AudioUncertainty float64 `json:"audio_uncertainty_seconds"`
}
type DecodedMedia struct {
	Name               string             `json:"name"`
	Duration           float64            `json:"duration_seconds"`
	Streams            []StreamIdentity   `json:"streams"`
	Video              []VisualEvent      `json:"visual_events,omitempty"`
	Audio              []AcousticEvidence `json:"acoustic_events,omitempty"`
	VideoPTS           []float64          `json:"video_frame_pts_seconds,omitempty"`
	AudioFrames        []AudioFrame       `json:"audio_frame_pts,omitempty"`
	DecodedVideoSHA256 string             `json:"decoded_video_sha256,omitempty"`
	DecodedAudioSHA256 string             `json:"decoded_audio_sha256,omitempty"`
}
type TimingManifest struct {
	SchemaVersion    int             `json:"schema_version"`
	Status           string          `json:"status"`
	Recipe           Recipe          `json:"recipe"`
	Tools            []ToolIdentity  `json:"tools"`
	Media            []MediaIdentity `json:"media"`
	Commands         []ToolCommand   `json:"commands"`
	Decoded          []DecodedMedia  `json:"decoded"`
	Events           []DecodedEvent  `json:"events"`
	PhysicalEvidence string          `json:"physical_evidence"`
}

func writeJSONFile(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0600)
}
func validateDestination(destination string) (string, error) {
	if destination == "" {
		return "", errors.New("explicit new destination required")
	}
	abs, err := filepath.Abs(destination)
	if err != nil {
		return "", err
	}
	if _, err := os.Lstat(abs); !os.IsNotExist(err) {
		return "", errors.New("destination already exists or is inaccessible")
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return "", errors.New("destination parent must exist")
	}
	resolved := filepath.Join(parent, filepath.Base(abs))
	// This reserved source subtree is protected even when the caller's working
	// directory is a package, or a destination parent resolves through a symlink.
	if strings.Contains(filepath.ToSlash(resolved)+"/", "/experiments/playback/testdata/") {
		return "", errors.New("original fixture directory is protected")
	}
	return resolved, nil
}
func Generate(ctx context.Context, options GenerateOptions) (*TimingManifest, error) {
	if ctx == nil {
		return nil, errors.New("context required")
	}
	destination, err := validateDestination(options.Destination)
	if err != nil {
		return nil, err
	}
	if options.Timeout == 0 {
		options.Timeout = 180 * time.Second
	}
	if options.Timeout <= 0 || options.Timeout > 600*time.Second {
		return nil, errors.New("generation timeout must be within 600 seconds")
	}
	ctx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()
	if options.FFmpeg == "" {
		options.FFmpeg = "ffmpeg"
	}
	if options.FFprobe == "" {
		options.FFprobe = "ffprobe"
	}
	ffmpeg, err := identifyTool(ctx, options.FFmpeg)
	if err != nil {
		return nil, err
	}
	ffprobe, err := identifyTool(ctx, options.FFprobe)
	if err != nil {
		return nil, err
	}
	stage, err := os.MkdirTemp(filepath.Dir(destination), ".output-fixtures-stage-")
	if err != nil {
		return nil, err
	}
	m := &TimingManifest{SchemaVersion: 1, Status: "incomplete", Recipe: DefaultRecipe(), Tools: []ToolIdentity{ffmpeg, ffprobe}, PhysicalEvidence: "not-tested; fixture decoding is not physical playback evidence"}
	success := false
	defer func() {
		if success {
			_ = os.RemoveAll(stage)
		} else {
			_ = writeJSON(filepath.Join(stage, "failed-timing.json"), m)
		}
	}()
	run := func(tool string, args []string, limit int) ([]byte, error) {
		b, stderr, err := runTool(ctx, tool, args, limit)
		c := ToolCommand{Tool: filepath.Base(tool), Arguments: append([]string(nil), args...), StdoutBytes: len(b), Stderr: stderr}
		for i := range c.Arguments {
			c.Arguments[i] = strings.ReplaceAll(c.Arguments[i], stage, "<staging>")
		}
		if err != nil {
			c.Error = err.Error()
		}
		m.Commands = append(m.Commands, c)
		if err != nil {
			return b, toolError(tool, err)
		}
		return b, nil
	}
	for _, item := range []struct {
		name  string
		write func(*os.File) error
	}{{"source.gray", func(f *os.File) error { return WriteVideo(f) }}, {"source.s16le", func(f *os.File) error { return WriteAudio(f) }}} {
		f, e := os.OpenFile(filepath.Join(stage, item.name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return m, e
		}
		e = item.write(f)
		closeErr := f.Close()
		if e != nil {
			return m, e
		}
		if closeErr != nil {
			return m, closeErr
		}
	}
	common := []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-n"}
	encode := append(append([]string{}, common...), "-f", "rawvideo", "-pixel_format", "gray", "-video_size", "640x360", "-framerate", "30", "-i", filepath.Join(stage, "source.gray"), "-f", "s16le", "-ar", "48000", "-ac", "1", "-i", filepath.Join(stage, "source.s16le"), "-map", "0:v:0", "-map", "1:a:0", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-preset", "veryfast", "-c:a", "aac", "-b:a", "128k", "-movflags", "+faststart", "-shortest", filepath.Join(stage, "combined.mp4"))
	if _, err = run(ffmpeg.Path, encode, 64<<10); err != nil {
		return m, fmt.Errorf("generation failed (preserved stage %s): %w", stage, err)
	}
	for _, r := range []struct{ name, selector string }{{"video.mp4", "0:v:0"}, {"audio.m4a", "0:a:0"}} {
		args := append(append([]string{}, common...), "-i", filepath.Join(stage, "combined.mp4"), "-map", r.selector, "-c", "copy", "-movflags", "+faststart", filepath.Join(stage, r.name))
		if _, err = run(ffmpeg.Path, args, 64<<10); err != nil {
			return m, err
		}
	}
	for _, name := range []string{"combined.mp4", "video.mp4", "audio.m4a"} {
		h, n, e := hashRegular(filepath.Join(stage, name), MaxMediaBytes)
		if e != nil {
			return m, e
		}
		m.Media = append(m.Media, MediaIdentity{name, h, n})
		d, e := decodeMedia(stage, name, ffmpeg.Path, ffprobe.Path, run)
		if e != nil {
			return m, fmt.Errorf("decoded validation failed for %s (preserved stage %s): %w", name, stage, e)
		}
		m.Decoded = append(m.Decoded, d)
	}
	if err = validateDecoded(m); err != nil {
		return m, fmt.Errorf("fixture validation failed (preserved stage %s): %w", stage, err)
	}
	m.Status = "validated-fixture-only"
	if err = writeJSON(filepath.Join(stage, "manifest.json"), ServerManifest{"Original offline paired geometry and acoustic IDs; no third-party media", m.Media}); err != nil {
		return m, err
	}
	if err = writeJSON(filepath.Join(stage, TimingManifestName), m); err != nil {
		return m, err
	}
	if err := ctx.Err(); err != nil {
		return m, err
	}
	// Reserve a new destination exclusively only after complete validation. The
	// eligibility manifest is moved last; no existing directory is replaced.
	if err = os.Mkdir(destination, 0700); err != nil {
		return m, errors.New("destination appeared before publication")
	}
	for _, name := range []string{"combined.mp4", "video.mp4", "audio.m4a", TimingManifestName, "manifest.json"} {
		if err = os.Rename(filepath.Join(stage, name), filepath.Join(destination, name)); err != nil {
			return m, err
		}
	}
	success = true
	return m, nil
}
