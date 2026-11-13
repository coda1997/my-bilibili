package outputprobe

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"testing"
)

func TestRecipeRationalEventsAndVisualIDs(t *testing.T) {
	r := DefaultRecipe()
	if len(r.Events) != 6 || len(r.Bits) != 3 {
		t.Fatal("incomplete recipe")
	}
	var raw bytes.Buffer
	if err := WriteVideo(&raw); err != nil {
		t.Fatal(err)
	}
	if raw.Len() != Width*Height*FrameRate*Seconds {
		t.Fatal("incorrect video dimensions")
	}
	pts := make([]float64, FrameRate*Seconds)
	for i := range pts {
		pts[i] = float64(i) / FrameRate
	}
	events, err := detectVisual(raw.Bytes(), pts)
	if err != nil || len(events) != 6 {
		t.Fatalf("%v %+v", err, events)
	}
	for i, e := range events {
		want := r.Events[i]
		if e.ID != i+1 || e.FirstFrame != want.FirstFrame || e.EndFrame != want.EndFrame || want.FirstSample/SampleRate != e.ID || want.EndSample-want.FirstSample != SampleRate/10 {
			t.Fatalf("recipe/time mismatch: %+v %+v", e, want)
		}
	}
	raw.Bytes()[FrameRate*Width*Height+280*Width+140] = 40
	if _, err := detectVisual(raw.Bytes(), pts); err == nil {
		t.Fatal("unrecognized marker accepted")
	}
}
func TestPCMRecipeEncodingAndPeakBound(t *testing.T) {
	samples := SourcePCM()
	var raw bytes.Buffer
	if err := WriteAudio(&raw); err != nil {
		t.Fatal(err)
	}
	if raw.Len() != Seconds*SampleRate*2 {
		t.Fatal("incorrect PCM length")
	}
	for i, v := range samples {
		if math.Abs(v) > 0.32 {
			t.Fatal("amplitude bound exceeded")
		}
		decoded := float64(int16(binary.LittleEndian.Uint16(raw.Bytes()[i*2:]))) / 32767
		if math.Abs(decoded-v) > 1.0/32767 {
			t.Fatal("PCM quantization bound exceeded")
		}
	}
	if err := WriteAudio(errorWriter{}); err == nil {
		t.Fatal("write failure hidden")
	}
	if err := WriteVideo(errorWriter{}); err == nil {
		t.Fatal("video write failure hidden")
	}
}

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
