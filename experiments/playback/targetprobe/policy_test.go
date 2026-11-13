package targetprobe

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"my-bilibili/experiments/playback/contract"
)

func validDescriptor(now time.Time) descriptor {
	return descriptor{
		Version: 2, Source: Source{"BV1Zs411o7Ee", 899574, 1445374, 1},
		Rendition: Rendition{32, "H264/AVC", 30216, "AAC-LC"}, Grant: Grant{"full", 0, 596624},
		ObservedAt: now, FreshUntil: now.Add(SessionLimit),
		VideoURL: "https://video.example.test/synthetic?sign=PRIVATE_SYNTHETIC%2B%2F",
		AudioURL: "https://audio.example.test/synthetic?sign=PRIVATE_SYNTHETIC",
		Headers:  map[string]string{"Referer": PublicReferer, "User-Agent": PublicUserAgent},
	}
}
func encoded(d any) []byte { b, _ := json.Marshal(d); return b }
func testPolicy(t *testing.T, now *time.Time) *Policy {
	t.Helper()
	p, e := Parse(encoded(validDescriptor(*now)), func() time.Time { return *now })
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestStrictDescriptorAndPrivacy(t *testing.T) {
	now := time.Now()
	b := encoded(validDescriptor(now))
	p, e := Parse(b, func() time.Time { return now })
	if e != nil {
		t.Fatal(e)
	}
	if p.Metadata().Policy != PolicyID {
		t.Fatal("wrong policy")
	}
	if strings.Contains(string(encoded(p.Metadata())), "PRIVATE_SYNTHETIC") {
		t.Fatal("locator leaked")
	}
	if p.Input().URL != validDescriptor(now).VideoURL {
		t.Fatal("query modified")
	}
	for _, mutate := range []func([]byte) []byte{
		func(b []byte) []byte { return append(b, []byte(`{}`)...) },
		func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"version":2`), []byte(`"version":2,"version":2`), 1)
		},
		func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"page":1`), []byte(`"page":1,"\u0070age":1`), 1)
		},
		func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"version":2`), []byte(`"version":2,"unknown":"PRIVATE_SYNTHETIC"`), 1)
		},
		func(b []byte) []byte { return bytes.Replace(b, []byte(`"page":1`), []byte(`"page":1,"unknown":0`), 1) },
		func(b []byte) []byte { return bytes.Replace(b, []byte(`"page":1`), []byte(`"page":null`), 1) },
		func(b []byte) []byte { return bytes.Replace(b, []byte(`"start_ms":0`), []byte(`"start_ms":null`), 1) },
		func(b []byte) []byte { return bytes.Replace(b, []byte(`"version":2`), []byte(`"version":2.0`), 1) },
		func(b []byte) []byte { return bytes.Replace(b, []byte(`"version":2`), []byte(`"version":2e999`), 1) },
		func(b []byte) []byte { return append(b, bytes.Repeat([]byte(" "), DescriptorLimit)...) },
	} {
		if _, e := Parse(mutate(bytes.Clone(b)), func() time.Time { return now }); e != contract.Invalid {
			t.Fatal("malformed descriptor accepted or unsafe error", e)
		}
	}
	var root map[string]json.RawMessage
	_ = json.Unmarshal(b, &root)
	for key := range root {
		m := map[string]json.RawMessage{}
		for k, v := range root {
			if k != key {
				m[k] = v
			}
		}
		if _, e := Parse(encoded(m), func() time.Time { return now }); e != contract.Invalid {
			t.Fatal("missing field", key, e)
		}
	}
}
func TestTrustedIdentityRoleHeadersAndExpiry(t *testing.T) {
	now := time.Now()
	for name, mutate := range map[string]func(*descriptor){
		"source": func(d *descriptor) { d.Source.CID++ }, "rendition": func(d *descriptor) { d.Rendition.AudioID++ }, "grant": func(d *descriptor) { d.Grant.EndMS++ }, "preview": func(d *descriptor) { d.Grant.Kind = "preview" },
		"credentials": func(d *descriptor) { d.VideoURL = "https://user:pass@video.example.test/x" }, "http": func(d *descriptor) { d.VideoURL = strings.Replace(d.VideoURL, "https:", "http:", 1) },
		"cookie": func(d *descriptor) { d.Headers["Cookie"] = "PRIVATE_SYNTHETIC" }, "header-value": func(d *descriptor) { d.Headers["Referer"] = "PRIVATE_SYNTHETIC" }, "duplicate-header": func(d *descriptor) { d.Headers["referer"] = PublicReferer },
		"header-injection": func(d *descriptor) { d.Headers["User-Agent"] += "\r\nX: PRIVATE_SYNTHETIC" }, "future": func(d *descriptor) { d.ObservedAt = now.Add(time.Second) },
		"freshness-extension": func(d *descriptor) { d.FreshUntil = now.Add(91 * time.Second) }, "service-expiry": func(d *descriptor) { v := now.Add(time.Second); d.ServiceExpiresAt = &v },
		"absent-audio": func(d *descriptor) { d.AudioURL = "" }, "oversize-url": func(d *descriptor) { d.VideoURL += strings.Repeat("a", 4096) },
	} {
		t.Run(name, func(t *testing.T) {
			d := validDescriptor(now)
			mutate(&d)
			_, e := Parse(encoded(d), func() time.Time { return now })
			if e != contract.Invalid {
				t.Fatal(e)
			}
		})
	}
	d := validDescriptor(now)
	d.FreshUntil = now.Add(time.Second)
	p, e := Parse(encoded(d), func() time.Time { return now })
	if e != nil {
		t.Fatal(e)
	}
	now = now.Add(time.Second)
	if p.Validate(p.Input()) != contract.Timeout {
		t.Fatal("expired input accepted")
	}
	if _, e := Parse(encoded(d), func() time.Time { return now }); e != contract.Timeout {
		t.Fatal("expired parse", e)
	}
}
func TestPolicyCopiesAndRejectsChangedInput(t *testing.T) {
	now := time.Now()
	d := validDescriptor(now)
	known := now.Add(SessionLimit)
	d.ServiceExpiresAt = &known
	p, e := Parse(encoded(d), func() time.Time { return now })
	if e != nil {
		t.Fatal(e)
	}
	in := p.Input()
	in.Headers["Referer"] = "changed"
	if p.Validate(in) != contract.Invalid || p.Validate(p.Input()) != nil {
		t.Fatal("mutated metadata accepted or retained")
	}
	m := p.Metadata()
	*m.ServiceExpiresAt = now.Add(-time.Hour)
	if p.Validate(p.Input()) != nil {
		t.Fatal("metadata pointer escaped")
	}
	in = p.Input()
	in.URL += "changed"
	if p.Validate(in) != contract.Invalid {
		t.Fatal("locator changed")
	}
	in = p.Input()
	in.FilePath = "local"
	if p.Validate(in) != contract.Invalid {
		t.Fatal("file path accepted")
	}
}
func TestLocatorUnicodeAndEscapes(t *testing.T) {
	for _, suffix := range []string{"?x=%zz", "?x=%00", "%0a", "?x=%C2%85", "%C2%85", "?x=%ff", "%ff", "?x=\u0085", "#", "?x=\r"} {
		if e := ValidateMediaURL("https://video.example.test/x" + suffix); e != contract.Invalid {
			t.Fatal("bad locator accepted")
		}
	}
	for _, u := range []string{"https:///x", "https://:443/x", "https://host.example:/x", "https://host.example:0/x", "https://host.example:65536/x", "https://host.example:abc/x", "https://[bad]/x", "https://[127.0.0.1]/x", "https://::1/x", "https://%C2%85.example/x", "https://%ff.example/x"} {
		if e := ValidateMediaURL(u); e != contract.Invalid {
			t.Fatal("malformed URL accepted")
		}
	}
	if e := ValidateMediaURL("https://video.example.test/中文?k=%E4%B8%AD&x=%2F%2B"); e != nil {
		t.Fatal(e)
	}
}

type blockingReader struct{ closed chan struct{} }

func (r *blockingReader) Read([]byte) (int, error) { <-r.closed; return 0, io.EOF }
func (r *blockingReader) Close() error {
	select {
	case <-r.closed:
	default:
		close(r.closed)
	}
	return nil
}
func TestBoundedInputDeadlineAndClosure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, e := ReadDescriptor(ctx, io.NopCloser(strings.NewReader(strings.Repeat("x", DescriptorLimit+1)))); e != contract.Invalid {
		t.Fatal(e)
	}
	r := &blockingReader{make(chan struct{})}
	ctx2, cancel2 := context.WithCancel(ctx)
	cancel2()
	if _, e := ReadDescriptor(ctx2, r); e != contract.Canceled {
		t.Fatal(e)
	}
	select {
	case <-r.closed:
	default:
		t.Fatal("input not closed")
	}
	ctx3, cancel3 := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel3()
	if _, e := ReadDescriptor(ctx3, io.NopCloser(strings.NewReader("x"))); e != contract.Timeout {
		t.Fatal(e)
	}
}
