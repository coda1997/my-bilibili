package targetprobe

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"my-bilibili/experiments/playback/contract"
)

func TestTargetRejectsLegacyOriginDescriptor(t *testing.T) {
	now := time.Now()
	d := validDescriptor(now)
	d.Version = 1
	if _, err := Parse(encoded(d), func() time.Time { return now }); err != contract.Invalid {
		t.Fatal("legacy descriptor version accepted", err)
	}
	d.Version = 2
	var fields map[string]any
	if err := json.Unmarshal(encoded(d), &fields); err != nil {
		t.Fatal(err)
	}
	fields["origins"] = map[string]string{"video": "https://video.example.test:443", "audio": "https://audio.example.test:443"}
	if _, err := Parse(encoded(fields), func() time.Time { return now }); err != contract.Invalid {
		t.Fatal("removed origin declaration accepted", err)
	}
}

func TestTargetAcceptsAllHTTPSHosts(t *testing.T) {
	for _, host := range []string{
		"unseen.example.test", "audio.example.test:8443", "localhost", "a.localhost",
		"media", "CDN.Example.Test.", "127.0.0.1:9443", "192.168.1.2", "[::1]:8443",
		"[2001:db8::1]", "[fe80::1%25en0]:443", "example.test:1", "example.test:65535",
	} {
		t.Run(host, func(t *testing.T) {
			now := time.Now()
			d := validDescriptor(now)
			d.VideoURL = "HTTPS://" + host + "/synthetic-video?sign=PRIVATE_SYNTHETIC%2B%2f&x=two+words"
			d.AudioURL = "https://" + host + "/synthetic-audio?sign=PRIVATE_SYNTHETIC%2F"
			p, err := Parse(encoded(d), func() time.Time { return now })
			if err != nil {
				t.Fatal("valid HTTPS host rejected", err)
			}
			in := p.Input()
			if in.URL != d.VideoURL || in.AudioURL != d.AudioURL || p.Validate(in) != nil {
				t.Fatal("locator bytes changed or valid input rejected")
			}
			public := string(encoded(p.Metadata()))
			for _, private := range []string{host, "PRIVATE_SYNTHETIC", "synthetic-video", "origins", "video_url", "audio_url"} {
				if strings.Contains(public, private) {
					t.Fatal("locator or host appeared in public metadata")
				}
			}
			// Removing destination restrictions must not permit replacing a
			// previously accepted input after its descriptor has been parsed.
			in.URL = "https://another.example.test/video"
			if p.Validate(in) != contract.Invalid {
				t.Fatal("post-validation locator replacement accepted")
			}
			in = p.Input()
			in.URL, in.AudioURL = in.AudioURL, in.URL
			if p.Validate(in) != contract.Invalid {
				t.Fatal("post-validation track replacement accepted")
			}
		})
	}
}
