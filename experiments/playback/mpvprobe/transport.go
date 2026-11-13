package mpvprobe

import (
	"runtime"
	"sort"
	"strings"
	"unsafe"

	"my-bilibili/experiments/playback/contract"
)

// mpv_node's C union is eight bytes, followed by a four-byte format and padding.
// Pointer members are accessed as pointers, never converted through uintptr.
// KeepAlive below retains every Go allocation hidden behind the C union until
// mpv_set_option has copied it. Returned nodes belong to mpv until freeNode.
type nodeValue struct {
	Value  uint64
	Format int32
	_      int32
}
type nodeList struct {
	Count  int32
	_      int32
	Values *nodeValue
	Keys   **byte
}

func (n *nodeValue) pointer() unsafe.Pointer { return *(*unsafe.Pointer)(unsafe.Pointer(&n.Value)) }
func (n *nodeValue) setPointer(p unsafe.Pointer) {
	*(*unsafe.Pointer)(unsafe.Pointer(&n.Value)) = p
}

func (a *nativeAPI) stringList(core uintptr, name string, values []string) int32 {
	storage := make([][]byte, len(values))
	nodes := make([]nodeValue, len(values))
	for i, value := range values {
		storage[i] = append([]byte(value), 0)
		nodes[i].Format = 1 // MPV_FORMAT_STRING
		nodes[i].setPointer(unsafe.Pointer(&storage[i][0]))
	}
	list := nodeList{Count: int32(len(nodes))}
	if len(nodes) > 0 {
		list.Values = &nodes[0]
	}
	n := nodeValue{Format: 7} // MPV_FORMAT_NODE_ARRAY
	n.setPointer(unsafe.Pointer(&list))
	result := a.optionNode(core, name, 6, unsafe.Pointer(&n))
	runtime.KeepAlive(storage)
	runtime.KeepAlive(nodes)
	runtime.KeepAlive(list)
	return result
}

func (a *nativeAPI) transport(core uintptr, in contract.Input) bool {
	keys := make([]string, 0, len(in.Headers))
	for key := range in.Headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var fields []string
	for _, key := range keys {
		if strings.EqualFold(key, "Referer") {
			if a.option(core, "referrer", in.Headers[key]) < 0 {
				return false
			}
		} else {
			fields = append(fields, key+": "+in.Headers[key])
		}
	}
	// Typed arrays preserve commas and path separators. No CLI list escaping,
	// parsed command text, singular audio-file alias or inherited session state.
	if len(fields) > 0 && a.stringList(core, "http-header-fields", fields) < 0 {
		return false
	}
	if in.AudioURL == "" {
		return true
	}
	// autoload-files=no also prevents selection of explicitly supplied tracks
	// in the pinned mpv. Permit selection for this required audio input; the
	// audio-file-auto, sub-auto and cover-art-auto options remain "no", so
	// companion-file discovery stays disabled. Missing audio still fails the
	// selected-track checks after loading instead of falling back to video only.
	return a.stringList(core, "audio-files", []string{in.AudioURL}) >= 0 &&
		a.option(core, "autoload-files", "yes") >= 0
}

// Tracks is sanitized selected-track evidence, not proof of audible/synced output.
type Tracks struct{ Video, Audio, ExternalAudio bool }

func Inspect(plain contract.Engine) Tracks {
	if e, ok := plain.(*engine); ok {
		e.mu.RLock()
		defer e.mu.RUnlock()
		return e.tracks
	}
	return Tracks{}
}

func nodeString(p *byte) (string, bool) {
	if p == nil {
		return "", false
	}
	var b []byte
	for i := 0; i < 128; i++ {
		v := *(*byte)(unsafe.Add(unsafe.Pointer(p), i))
		if v == 0 {
			return string(b), true
		}
		b = append(b, v)
	}
	return "", false
}

func selectedTracks(n *nodeValue) (Tracks, bool) {
	var result Tracks
	if n.Format != 7 || n.pointer() == nil {
		return result, false
	}
	list := (*nodeList)(n.pointer())
	if list.Count < 0 || list.Count > 32 || list.Count > 0 && list.Values == nil {
		return result, false
	}
	for i := range unsafe.Slice(list.Values, int(list.Count)) {
		item := &unsafe.Slice(list.Values, int(list.Count))[i]
		if item.Format != 8 || item.pointer() == nil {
			return Tracks{}, false
		}
		fields := (*nodeList)(item.pointer())
		if fields.Count < 0 || fields.Count > 64 || fields.Count > 0 && (fields.Values == nil || fields.Keys == nil) {
			return Tracks{}, false
		}
		kind, selected, external := "", false, false
		for j, v := range unsafe.Slice(fields.Values, int(fields.Count)) {
			key, ok := nodeString(unsafe.Slice(fields.Keys, int(fields.Count))[j])
			if !ok {
				return Tracks{}, false
			}
			switch key {
			case "type":
				if v.Format != 1 {
					return Tracks{}, false
				}
				kind, ok = nodeString((*byte)(v.pointer()))
				if !ok {
					return Tracks{}, false
				}
			case "selected", "external":
				if v.Format != 3 {
					return Tracks{}, false
				}
				flag := *(*int32)(unsafe.Pointer(&v.Value)) != 0
				if key == "selected" {
					selected = flag
				} else {
					external = flag
				}
			}
		}
		if selected {
			result.Video = result.Video || kind == "video"
			result.Audio = result.Audio || kind == "audio"
			result.ExternalAudio = result.ExternalAudio || kind == "audio" && external
		}
	}
	return result, true
}

func (a *nativeAPI) tracks(core uintptr) (Tracks, bool) {
	var n nodeValue
	if a.getProperty(core, "track-list", 6, unsafe.Pointer(&n)) < 0 {
		return Tracks{}, false
	}
	defer a.freeNode(unsafe.Pointer(&n))
	return selectedTracks(&n) // Copy booleans before native ownership is released.
}
