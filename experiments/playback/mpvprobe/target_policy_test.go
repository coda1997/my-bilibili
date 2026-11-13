package mpvprobe

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"unsafe"

	"my-bilibili/experiments/playback/contract"
)

type targetPolicyFunc func(contract.Input) error

func (f targetPolicyFunc) Validate(in contract.Input) error { return f(in) }
func remoteInput() contract.Input {
	return contract.Input{URL: "https://public.example.test/video?synthetic=one%2Btwo", AudioURL: "https://public.example.test/audio", Headers: map[string]string{"Referer": "https://public.example.test/", "User-Agent": "synthetic-public"}}
}

func TestTargetDualBoundaryRejectsBeforeNativeLoad(t *testing.T) {
	calls := 0
	loader := func(string) (*nativeAPI, error) { calls++; return nil, errors.New("PRIVATE_SYNTHETIC") }
	allow := targetPolicyFunc(func(contract.Input) error { return nil })
	deny := targetPolicyFunc(func(contract.Input) error { return contract.Timeout })
	for _, pair := range []struct {
		controller, opener contract.InputPolicy
		want               contract.ErrorCode
	}{{allow, nil, contract.Invalid}, {nil, allow, contract.Invalid}, {allow, deny, contract.Timeout}, {deny, allow, contract.Timeout}} {
		c := contract.NewWithPolicy(Opener{Policy: pair.opener, loader: loader}, nil, pair.controller)
		if err := c.Open(context.Background(), remoteInput()); err != pair.want {
			t.Fatal(err, pair.want)
		}
		_ = c.Close()
	}
	if calls != 0 {
		t.Fatal("rejection loaded native code")
	}
	for _, code := range []error{contract.Invalid, contract.Timeout, errors.New("PRIVATE_SYNTHETIC")} {
		_, err := (Opener{Policy: targetPolicyFunc(func(contract.Input) error { return code }), loader: loader}).Open(context.Background(), remoteInput())
		if err == nil || strings.Contains(err.Error(), "PRIVATE") {
			t.Fatal("unsafe rejection")
		}
	}
}
func TestTargetTLSBeforeInitializeAndFailureClosed(t *testing.T) {
	for _, reject := range []bool{false, true} {
		t.Run(map[bool]string{false: "enabled", true: "unsupported"}[reject], func(t *testing.T) {
			f := fake()
			tls, initialized := false, false
			f.a.option = func(_ uintptr, k, v string) int32 {
				if k == "tls-verify" {
					tls = v == "yes"
					if reject {
						return -5
					}
				}
				return 0
			}
			f.a.initialize = func(uintptr) int32 {
				if !tls {
					t.Error("initialized without explicit verification")
				}
				initialized = true
				return 0
			}
			var validated contract.Input
			in := remoteInput()
			entered, release := make(chan struct{}), make(chan struct{})
			e, err := (Opener{Policy: targetPolicyFunc(func(got contract.Input) error { validated = got; return nil }), loader: func(string) (*nativeAPI, error) { close(entered); <-release; return f.a, nil }}).Open(context.Background(), in)
			if err != nil {
				t.Fatal(err)
			}
			<-entered
			in.Headers["Referer"] = "PRIVATE_SYNTHETIC"
			validated.Headers["User-Agent"] = "PRIVATE_SYNTHETIC"
			old := f.a.option
			f.a.option = func(c uintptr, k, v string) int32 {
				if strings.Contains(v, "PRIVATE_SYNTHETIC") {
					t.Error("caller metadata reached worker")
				}
				return old(c, k, v)
			}
			close(release)
			native := e.(*engine)
			if reject {
				<-native.done
				if native.Snapshot().Error != contract.Unsupported || initialized {
					t.Fatal("TLS failure initialized or wrong code")
				}
			} else {
				select {
				case <-f.sent:
				case <-time.After(time.Second):
					t.Fatal("load absent")
				}
			}
			_ = e.Close()
			if Owned() != (Resources{}) {
				t.Fatal("resource leak")
			}
		})
	}
}
func TestTargetExpiredContextDoesNotLoad(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	calls := 0
	_, err := (Opener{Policy: targetPolicyFunc(func(contract.Input) error { return nil }), loader: func(string) (*nativeAPI, error) { calls++; return nil, nil }}).Open(ctx, remoteInput())
	if err != contract.Timeout || calls != 0 {
		t.Fatal(err, calls)
	}
}

// The parser accepts RFC-compatible scheme case. Native policy must treat every
// accepted spelling identically without normalizing the private locator itself.
func TestTargetHTTPSCaseTLSBoundaries(t *testing.T) {
	for _, scheme := range []string{"https", "HTTPS", "hTtPs"} {
		for _, reject := range []bool{false, true} {
			name := scheme + map[bool]string{false: "/enabled", true: "/unsupported"}[reject]
			t.Run(name, func(t *testing.T) {
				f := fake()
				in := remoteInput()
				in.URL = strings.Replace(in.URL, "https:", scheme+":", 1)
				in.AudioURL = strings.Replace(in.AudioURL, "https:", scheme+":", 1) + "?synthetic=one%2Btwo"
				tls, initialized := false, false
				f.a.option = func(_ uintptr, k, v string) int32 {
					if k == "tls-verify" {
						tls = v == "yes"
						if reject {
							return -5
						}
					}
					return 0
				}
				f.a.initialize = func(uintptr) int32 {
					if !tls {
						t.Error("accepted HTTPS initialized without explicit verification")
					}
					initialized = true
					return 0
				}
				command := f.a.command
				f.a.command = func(core uintptr, id uint64, p unsafe.Pointer) int32 {
					args := commandArgs(p)
					if len(args) > 1 && args[0] == "loadfile" && args[1] != in.URL {
						t.Error("video locator bytes modified")
					}
					return command(core, id, p)
				}
				f.a.optionNode = func(_ uintptr, name string, _ int32, p unsafe.Pointer) int32 {
					if name == "audio-files" {
						n := (*nodeValue)(p)
						list := (*nodeList)(n.pointer())
						values := unsafe.Slice(list.Values, int(list.Count))
						if len(values) != 1 {
							t.Error("audio locator count changed")
							return -1
						}
						value, ok := nodeString((*byte)(values[0].pointer()))
						if !ok || value != in.AudioURL {
							t.Error("audio locator bytes modified")
						}
					}
					return 0
				}
				plain, err := (Opener{Policy: targetPolicyFunc(func(contract.Input) error { return nil }), loader: func(string) (*nativeAPI, error) { return f.a, nil }}).Open(context.Background(), in)
				if err != nil {
					t.Fatal(err)
				}
				native := plain.(*engine)
				t.Cleanup(func() { _ = native.Close() })
				if reject {
					select {
					case <-native.done:
						if native.Snapshot().Error != contract.Unsupported || initialized {
							t.Error("TLS rejection initialized or lost unsupported code")
						}
					case <-f.sent:
						t.Error("TLS option rejection allowed load")
					case <-time.After(time.Second):
						t.Error("TLS rejection did not finish")
					}
				} else {
					select {
					case <-f.sent:
					case <-time.After(time.Second):
						t.Fatal("load absent")
					}
				}
				_ = native.Close()
				if Owned() != (Resources{}) {
					t.Fatal("resource leak")
				}
			})
		}
	}
}

func TestTargetHTTPSCaseCancellationClassification(t *testing.T) {
	for _, scheme := range []string{"https", "HTTPS", "hTtPs"} {
		t.Run(scheme, func(t *testing.T) {
			f := fake()
			in := remoteInput()
			in.URL = strings.Replace(in.URL, "https:", scheme+":", 1)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			plain, err := (Opener{Policy: targetPolicyFunc(func(contract.Input) error { return nil }), loader: func(string) (*nativeAPI, error) { return f.a, nil }}).Open(ctx, in)
			if err != nil {
				t.Fatal(err)
			}
			native := plain.(*engine)
			t.Cleanup(func() { _ = native.Close() })
			select {
			case <-f.sent:
			case <-time.After(time.Second):
				t.Fatal("load absent")
			}
			cancel()
			select {
			case <-native.done:
			case <-time.After(time.Second):
				t.Fatal("cancel did not finish")
			}
			if s := native.Snapshot(); s.State != contract.Failed || s.Error != contract.Canceled {
				t.Error("accepted HTTPS lost cancellation classification", s.State, s.Error)
			}
			_ = native.Close()
			if Owned() != (Resources{}) {
				t.Fatal("resource leak")
			}
		})
	}
}
