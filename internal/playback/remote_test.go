package playback

import (
	"context"
	"maps"
	"strings"
	"testing"
)

func publicRemoteInput() Input {
	return Input{URL: "https://new.example.test/video?synthetic=one%2Btwo", AudioURL: "https://another.example.test:8443/audio",
		Headers: map[string]string{"Referer": PublicReferer, "User-Agent": PublicUserAgent}}
}

func TestHTTPSPolicyKeepsAnyValidHostAndUnchangedInput(t *testing.T) {
	for _, raw := range []string{
		"https://new.example.test/video?one=two%20three", "HTTPS://cdn.example.test/video",
		"https://localhost:1/video", "https://127.0.0.1:65535/video",
		"https://192.168.1.2/video", "https://[::1]:443/video", "https://[fe80::1%25en0]/video",
		"https://host.example.test/path%20with%20space", "https://other.test/" + strings.Repeat("x", 4096-len("https://other.test/")),
	} {
		in := publicRemoteInput()
		in.URL = raw
		before := in
		before.Headers = maps.Clone(in.Headers)
		if err := (HTTPSPolicy{}).Validate(in); err != nil {
			t.Fatal("valid HTTPS rejected", raw, err)
		}
		if in.URL != before.URL || in.AudioURL != before.AudioURL || !maps.Equal(in.Headers, before.Headers) {
			t.Fatal("policy rewrote media")
		}
		in.AudioURL = ""
		if err := (HTTPSPolicy{}).Validate(in); err != nil {
			t.Fatal("silent video rejected")
		}
	}
}

func TestHTTPSPolicyRejectsMalformedOrPrivateMetadata(t *testing.T) {
	for _, raw := range []string{
		"", "http://host.test/video", "file:///tmp/video", "https:opaque", "https:///video",
		"https://user:secret@host.test/video", "https://host.test/video#fragment", "https://host.test/video#",
		"https://host.test/video\n", "https://host.test/video%0a", "https://host.test/video?q=%0a",
		"https://host.test/video%zz", "https://host.test/video?q=%zz", "https://host.test/a b",
		"https://host.test:/video", "https://host.test:0/video", "https://host.test:65536/video",
		"https://host.test:no/video", "https://[not-an-ip]/video", "https://::1/video",
		"https://host.test/" + strings.Repeat("x", 4097), "https://host.test/" + string([]byte{0xff}),
	} {
		in := publicRemoteInput()
		in.URL = raw
		if err := (HTTPSPolicy{}).Validate(in); err != Invalid {
			t.Fatal("invalid video allowed", raw, err)
		}
		if raw != "" {
			in = publicRemoteInput()
			in.AudioURL = raw
			if err := (HTTPSPolicy{}).Validate(in); err != Invalid {
				t.Fatal("invalid audio allowed", raw, err)
			}
		}
	}
	for _, change := range []func(*Input){
		func(in *Input) { in.FilePath = "/fixture.mp4" },
		func(in *Input) { in.Headers = nil },
		func(in *Input) { delete(in.Headers, "Referer") },
		func(in *Input) { in.Headers["Referer"] = "https://other.test/" },
		func(in *Input) { in.Headers["User-Agent"] = "other" },
		func(in *Input) { in.Headers["Cookie"] = "PRIVATE_SYNTHETIC" },
		func(in *Input) { in.Headers["Authorization"] = "PRIVATE_SYNTHETIC" },
		func(in *Input) { in.Headers["referer"] = PublicReferer },
		func(in *Input) { in.Headers["User-Agent"] += "\r\nInjected: bad" },
		func(in *Input) { in.Headers["Referer"] = strings.Repeat("x", 4097) },
	} {
		in := publicRemoteInput()
		change(&in)
		if err := (HTTPSPolicy{}).Validate(in); err != Invalid {
			t.Fatal("invalid metadata allowed", err)
		}
	}
}

func TestHTTPSPolicyRequiresControllerOptIn(t *testing.T) {
	calls := 0
	opener := openFunc(func(context.Context, Input) (Engine, error) {
		calls++
		return &fakeEngine{s: Snapshot{State: Ready}}, nil
	})
	in := publicRemoteInput()
	if err := New(opener, nil).Open(context.Background(), in); err != Invalid || calls != 0 {
		t.Fatal("default became remote")
	}
	c := NewWithPolicy(opener, nil, HTTPSPolicy{})
	defer c.Close()
	if err := c.Open(context.Background(), in); err != nil || calls != 1 {
		t.Fatal("explicit policy did not open", err)
	}
	in.Headers["Cookie"] = "PRIVATE_SYNTHETIC"
	if err := c.Open(context.Background(), in); err != Invalid || calls != 1 {
		t.Fatal("private metadata reached opener")
	}
}
