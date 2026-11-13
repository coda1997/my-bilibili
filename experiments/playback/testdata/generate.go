//go:build ignore

// Generate controlled media offline; this is not part of the player runtime.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type stream struct {
	Codec  string `json:"codec_name"`
	Type   string `json:"codec_type"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}
type probe struct {
	Streams []stream `json:"streams"`
	Format  struct {
		Duration string `json:"duration"`
	} `json:"format"`
}
type fixture struct {
	Name   string   `json:"name"`
	SHA256 string   `json:"sha256"`
	Bytes  int      `json:"bytes"`
	Args   []string `json:"ffmpeg_args"`
	Probe  probe    `json:"probe"`
}

func command(ctx context.Context, name string, args ...string) []byte {
	var stderr bytes.Buffer
	c := exec.CommandContext(ctx, name, args...)
	c.Stderr = &stderr
	b, err := c.Output()
	if err != nil {
		panic(fmt.Sprintf("fixture tool %s failed: %v: %s", name, err, stderr.String()))
	}
	return b
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	dir := "experiments/playback/testdata"
	tmp, err := os.MkdirTemp(dir, ".generate-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmp)
	base := []string{"-hide_banner", "-loglevel", "error", "-y"}
	type recipe struct {
		name string
		args []string
	}
	recipes := []recipe{
		{"combined.mp4", []string{"-f", "lavfi", "-i", "color=c=0x20c060:s=640x360:r=30:d=8,drawbox=x=80:y=70:w=160:h=120:color=0xf03030:t=fill,drawbox=x=380:y=160:w=180:h=80:color=0x3060f0:t=fill", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=8", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-preset", "veryfast", "-c:a", "aac", "-b:a", "64k", "-movflags", "+faststart", "-shortest"}},
		{"video.mp4", []string{"-i", filepath.Join(tmp, "combined.mp4"), "-an", "-c:v", "copy", "-movflags", "+faststart"}},
		{"audio.m4a", []string{"-i", filepath.Join(tmp, "combined.mp4"), "-vn", "-c:a", "copy", "-movflags", "+faststart"}},
	}
	manifest := struct {
		Origin   string    `json:"origin"`
		FFmpeg   string    `json:"ffmpeg"`
		FFprobe  string    `json:"ffprobe"`
		Fixtures []fixture `json:"fixtures"`
	}{Origin: "Original offline synthetic geometry and tone; no third-party media", FFmpeg: strings.Split(string(command(ctx, "ffmpeg", "-version")), "\n")[0], FFprobe: strings.Split(string(command(ctx, "ffprobe", "-version")), "\n")[0]}
	for _, r := range recipes {
		args := append(append(append([]string{}, base...), r.args...), filepath.Join(tmp, r.name))
		command(ctx, "ffmpeg", args...)
		b, err := os.ReadFile(filepath.Join(tmp, r.name))
		if err != nil {
			panic(err)
		}
		if len(b) > 2<<20 {
			panic("fixture exceeds size bound")
		}
		var p probe
		if err := json.Unmarshal(command(ctx, "ffprobe", "-v", "error", "-show_entries", "stream=codec_name,codec_type,width,height:format=duration", "-of", "json", filepath.Join(tmp, r.name)), &p); err != nil {
			panic(err)
		}
		d, err := time.ParseDuration(p.Format.Duration + "s")
		if err != nil || d > 10*time.Second || d <= 0 {
			panic("invalid fixture duration")
		}
		for _, s := range p.Streams {
			if s.Width > 640 || s.Height > 360 {
				panic("fixture exceeds dimensions")
			}
		}
		h := sha256.Sum256(b)
		for i := range args {
			args[i] = strings.ReplaceAll(args[i], tmp, "experiments/playback/testdata")
		}
		manifest.Fixtures = append(manifest.Fixtures, fixture{r.name, hex.EncodeToString(h[:]), len(b), args, p})
	}
	for _, r := range recipes {
		if err := os.Rename(filepath.Join(tmp, r.name), filepath.Join(dir, r.name)); err != nil {
			panic(err)
		}
	}
	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), append(b, '\n'), 0644); err != nil {
		panic(err)
	}
	fmt.Println("Generated three original fixtures and manifest.")
}
