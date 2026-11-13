package contract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFixtureManifest(t *testing.T) {
	var manifest struct {
		Fixtures []struct {
			Name, SHA256 string
			Bytes        int
			Probe        struct {
				Streams []struct {
					Codec         string `json:"codec_name"`
					Type          string `json:"codec_type"`
					Width, Height int
				}
				Format struct{ Duration string }
			}
		}
	}
	b, err := os.ReadFile("../testdata/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &manifest); err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"combined.mp4": 2, "video.mp4": 1, "audio.m4a": 1}
	if len(manifest.Fixtures) != len(want) {
		t.Fatal("unexpected fixture count")
	}
	for _, f := range manifest.Fixtures {
		n, ok := want[f.Name]
		if !ok {
			t.Fatal("unexpected fixture")
		}
		delete(want, f.Name)
		b, err := os.ReadFile(filepath.Join("../testdata", f.Name))
		if err != nil {
			t.Fatal(err)
		}
		h := sha256.Sum256(b)
		if hex.EncodeToString(h[:]) != f.SHA256 || len(b) != f.Bytes || len(b) > 2<<20 {
			t.Fatal("fixture hash/size mismatch")
		}
		d, err := time.ParseDuration(f.Probe.Format.Duration + "s")
		if err != nil || d <= 0 || d > 10*time.Second {
			t.Fatal("fixture duration")
		}
		if len(f.Probe.Streams) != n {
			t.Fatal("fixture tracks")
		}
		for _, s := range f.Probe.Streams {
			if s.Type == "video" && (s.Codec != "h264" || s.Width != 640 || s.Height != 360) || s.Type == "audio" && s.Codec != "aac" {
				t.Fatal("fixture codecs/dimensions")
			}
		}
	}
}
