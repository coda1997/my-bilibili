package mpvprobe

import (
	"context"
	"reflect"
	"runtime"
	"sync"
	"testing"
	"time"
	"unsafe"

	"my-bilibili/experiments/playback/contract"
)

// Own every allocation referenced through the C union throughout each fake call.
type trackFixture struct {
	root  nodeValue
	lists []*nodeList
	nodes [][]nodeValue
	text  [][]byte
	keys  [][]*byte
}

func trackNode(video, audio, external bool) *trackFixture {
	f := &trackFixture{}
	text := func(s string) *byte { b := append([]byte(s), 0); f.text = append(f.text, b); return &b[0] }
	var tracks []nodeValue
	for _, kind := range []string{"video", "audio"} {
		if kind == "video" && !video || kind == "audio" && !audio {
			continue
		}
		values := make([]nodeValue, 3)
		values[0].Format = 1
		values[0].setPointer(unsafe.Pointer(text(kind)))
		values[1].Format = 3
		values[1].Value = 1
		values[2].Format = 3
		if kind == "audio" && external {
			values[2].Value = 1
		}
		keys := []*byte{text("type"), text("selected"), text("external")}
		list := &nodeList{Count: 3, Values: &values[0], Keys: &keys[0]}
		n := nodeValue{Format: 8}
		n.setPointer(unsafe.Pointer(list))
		tracks = append(tracks, n)
		f.nodes = append(f.nodes, values)
		f.keys = append(f.keys, keys)
		f.lists = append(f.lists, list)
	}
	list := &nodeList{Count: int32(len(tracks))}
	if len(tracks) > 0 {
		list.Values = &tracks[0]
	}
	f.nodes = append(f.nodes, tracks)
	f.lists = append(f.lists, list)
	f.root.Format = 7
	f.root.setPointer(unsafe.Pointer(list))
	return f
}

func TestNodeABIAndSelectedTrackCopy(t *testing.T) {
	if unsafe.Sizeof(nodeValue{}) != 16 || unsafe.Offsetof(nodeValue{}.Format) != 8 || unsafe.Sizeof(nodeList{}) != 24 || unsafe.Offsetof(nodeList{}.Values) != 8 || unsafe.Offsetof(nodeList{}.Keys) != 16 {
		t.Fatal("node ABI mismatch")
	}
	f := trackNode(true, true, true)
	got, ok := selectedTracks(&f.root)
	if !ok || got != (Tracks{true, true, true}) {
		t.Fatal(got, ok)
	}
	// An unselected external audio track must not qualify as consumed audio.
	f.nodes[1][1].Value = 0
	got, ok = selectedTracks(&f.root)
	if !ok || got != (Tracks{Video: true}) {
		t.Fatal(got, ok)
	}
	f.lists[len(f.lists)-1].Count = 33
	if _, ok = selectedTracks(&f.root); ok {
		t.Fatal("oversized track list accepted")
	}
	runtime.KeepAlive(f)
}

func TestTypedListsCopyHeadersBeforeAsyncInitialization(t *testing.T) {
	f := fake()
	entered, release := make(chan struct{}), make(chan struct{})
	var mu sync.Mutex
	options := map[string][]string{}
	f.a.optionNode = func(_ uintptr, name string, format int32, p unsafe.Pointer) int32 {
		if format != 6 {
			t.Error("non-node option")
		}
		n := (*nodeValue)(p)
		list := (*nodeList)(n.pointer())
		var values []string
		for _, v := range unsafe.Slice(list.Values, int(list.Count)) {
			s, ok := nodeString((*byte)(v.pointer()))
			if !ok || v.Format != 1 {
				t.Error("invalid string list")
			}
			values = append(values, s)
		}
		mu.Lock()
		options[name] = values
		mu.Unlock()
		return 0
	}
	var referrer string
	plainOption := f.a.option
	f.a.option = func(core uintptr, name, value string) int32 {
		if name == "referrer" {
			referrer = value
		}
		return plainOption(core, name, value)
	}
	in := contract.Input{URL: "http://127.0.0.1/video", AudioURL: "http://127.0.0.1/audio,one", Headers: map[string]string{"X-Test": "one,two", "rEfErEr": "http://127.0.0.1/ref,one"}}
	e, err := (Opener{loader: func(string) (*nativeAPI, error) { close(entered); <-release; return f.a, nil }}).Open(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	<-entered
	in.Headers["X-Test"] = "changed"
	delete(in.Headers, "rEfErEr")
	close(release)
	select {
	case <-f.sent:
	case <-time.After(time.Second):
		t.Fatal("load not sent")
	}
	_ = e.Close()
	mu.Lock()
	defer mu.Unlock()
	if !reflect.DeepEqual(options["http-header-fields"], []string{"X-Test: one,two"}) || !reflect.DeepEqual(options["audio-files"], []string{"http://127.0.0.1/audio,one"}) || referrer != "http://127.0.0.1/ref,one" {
		t.Fatal("metadata changed, split or dropped")
	}
}

func TestMissingAudioFailsAndDisposesBeforeTerminalState(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		video, audio, external bool
		failed                 bool
	}{
		{"selected-external", true, true, true, false}, {"no-audio", true, false, false, true}, {"embedded-only", true, true, false, true}, {"no-video", false, true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := fake()
			tracks := trackNode(tc.video, tc.audio, tc.external)
			freed := false
			f.a.getProperty = func(_ uintptr, name string, format int32, p unsafe.Pointer) int32 {
				if name != "track-list" || format != 6 {
					t.Error("unexpected property")
				}
				*(*nodeValue)(p) = tracks.root
				return 0
			}
			f.a.freeNode = func(unsafe.Pointer) { freed = true }
			plain, err := (Opener{loader: func(string) (*nativeAPI, error) { return f.a, nil }}).Open(context.Background(), contract.Input{URL: "http://127.0.0.1/video", AudioURL: "http://127.0.0.1/audio"})
			if err != nil {
				t.Fatal(err)
			}
			e := plain.(*engine)
			defer e.Close()
			select {
			case <-f.sent:
			case <-time.After(time.Second):
				t.Fatal("load not sent")
			}
			f.events <- &event{ID: 5, Reply: 1}
			f.events <- &event{ID: 8}
			if tc.failed {
				select {
				case <-e.done:
				case <-time.After(time.Second):
					t.Fatal("missing track did not dispose")
				}
				if e.Snapshot().Error != contract.Media || Owned() != (Resources{}) || e.Command(contract.Command{Kind: contract.Play}) != contract.IsClosed {
					t.Fatal(e.Snapshot(), Owned())
				}
			} else {
				await(t, func() bool { return Inspect(e).ExternalAudio })
				_ = e.Close()
			}
			if !freed {
				t.Fatal("native property not freed")
			}
			runtime.KeepAlive(tracks)
		})
	}
}

func TestEventLossAndBudgetOverflowDispose(t *testing.T) {
	for _, id := range []int32{24, 1, 22} {
		t.Run(string(rune('a'+id)), func(t *testing.T) {
			f := fake()
			e := openFake(t, f)
			count := 1
			if id == 22 {
				count = 257
			}
			for range count {
				f.events <- &event{ID: id}
			}
			select {
			case <-e.done:
			case <-time.After(time.Second):
				t.Fatal("lost events not fatal")
			}
			if e.Snapshot().Error != contract.Media || Owned() != (Resources{}) {
				t.Fatal(e.Snapshot(), Owned())
			}
			// No registered native callbacks; events after disposal are never polled.
			f.events <- &event{ID: 8}
			_ = e.Close()
			_ = e.Close()
		})
	}
}

func TestLaterExternalAudioLossFailsWithoutRetainingNativePayload(t *testing.T) {
	f := fake()
	initial := trackNode(true, true, true)
	later := trackNode(true, false, false)
	f.a.getProperty = func(_ uintptr, _ string, _ int32, p unsafe.Pointer) int32 { *(*nodeValue)(p) = initial.root; return 0 }
	e0, err := (Opener{loader: func(string) (*nativeAPI, error) { return f.a, nil }}).Open(context.Background(), contract.Input{URL: "http://127.0.0.1/video", AudioURL: "http://127.0.0.1/audio"})
	if err != nil {
		t.Fatal(err)
	}
	e := e0.(*engine)
	defer e.Close()
	select {
	case <-f.sent:
	case <-time.After(time.Second):
		t.Fatal("load not sent")
	}
	f.events <- &event{ID: 5, Reply: 1}
	f.events <- &event{ID: 8}
	await(t, func() bool { return Inspect(e).ExternalAudio })
	p := property{Format: 6, Data: unsafe.Pointer(&later.root)}
	f.events <- &event{ID: 22, Reply: 7, Data: unsafe.Pointer(&p)}
	select {
	case <-e.done:
	case <-time.After(time.Second):
		t.Fatal("audio loss did not dispose")
	}
	if e.Snapshot().Error != contract.Media || Owned() != (Resources{}) {
		t.Fatal(e.Snapshot(), Owned())
	}
	runtime.KeepAlive(initial)
	runtime.KeepAlive(later)
}

func TestTransportConfigurationFailureDisposes(t *testing.T) {
	f := fake()
	f.a.optionNode = func(uintptr, string, int32, unsafe.Pointer) int32 { return -1 }
	plain, err := (Opener{loader: func(string) (*nativeAPI, error) { return f.a, nil }}).Open(context.Background(), contract.Input{URL: "http://127.0.0.1/video", Headers: map[string]string{"X-Test": "one,two"}})
	if err != nil {
		t.Fatal(err)
	}
	e := plain.(*engine)
	defer e.Close()
	select {
	case <-e.done:
	case <-time.After(time.Second):
		t.Fatal("option failure did not dispose")
	}
	if e.Snapshot().Error != contract.Media || Owned() != (Resources{}) {
		t.Fatal(e.Snapshot(), Owned())
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if !reflect.DeepEqual(f.order, []string{"destroy", "unload"}) {
		t.Fatal(f.order)
	}
}
