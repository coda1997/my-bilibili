package fixture

import (
	"testing"
	"time"

	"my-bilibili/internal/catalog"
	"my-bilibili/internal/danmaku"
	"my-bilibili/internal/playback"
)

func TestDanmakuFixturePartsAndOwnership(t *testing.T) {
	for part, title := range map[catalog.PartID]string{"part-1": "中文弹幕测试", "part-2": "第二分P弹幕"} {
		t.Run(string(part), func(t *testing.T) {
			entries, err := Danmaku(part)
			if err != nil || len(entries) != 64 || entries[0].Text != title || entries[0].At != 0 || entries[0].Mode != danmaku.FixedTop {
				t.Fatal("fixture part or opening phrase changed", err)
			}
			entries[0] = danmaku.Entry{Text: "changed by caller"}
			again, err := Danmaku(part)
			if err != nil || again[0].Text != title || again[0].Mode != danmaku.FixedTop {
				t.Fatal("caller mutation reached another fixture result", err)
			}
		})
	}
	for _, part := range []catalog.PartID{"", "part-0", "part-3", "unrelated-id"} {
		if entries, err := Danmaku(part); err != danmaku.Invalid || entries != nil {
			t.Fatal("unknown part acquired fixture comments")
		}
	}
}

func TestDanmakuFixturesProvideNativeProofAndDensity(t *testing.T) {
	// These comments are available independently of a resource directory.
	t.Chdir(t.TempDir())
	for _, part := range []catalog.PartID{"part-1", "part-2"} {
		entries, err := Danmaku(part)
		if err != nil {
			t.Fatal(err)
		}
		wantAt := []time.Duration{0, time.Second, 1500 * time.Millisecond, 1750 * time.Millisecond, 4500 * time.Millisecond, 4600 * time.Millisecond, 5 * time.Second, 9 * time.Second}
		for i, at := range wantAt {
			if entries[i].At != at {
				t.Fatal("native scenario timestamp changed", i, entries[i].At)
			}
		}
		if entries[1].Text != "暂停后位置不变" || entries[4].Text != "拖动后重新出现" {
			t.Fatal("native timing phrases missing")
		}
		modes := map[danmaku.Mode]bool{}
		for i, entry := range entries {
			if entry.At < 0 || entry.At >= 30*time.Second {
				t.Fatal("fixture comment outside its video")
			}
			if i >= 8 && entry.At != 12*time.Second+time.Duration((i-8)%4)*time.Millisecond {
				t.Fatal("dense fixture interval changed")
			}
			modes[entry.Mode] = true
		}
		for _, mode := range []danmaku.Mode{danmaku.ScrollRTL, danmaku.ScrollLTR, danmaku.FixedTop, danmaku.FixedBottom} {
			if !modes[mode] {
				t.Fatal("fixture mode missing", mode)
			}
		}
		timeline := danmaku.New()
		if err := timeline.Reset(1); err != nil {
			t.Fatal(err)
		}
		if err := timeline.SetEntries(1, entries); err != nil {
			t.Fatal("fixture data is not accepted by the frozen model", err)
		}
		sample := playback.Snapshot{Generation: 1, State: playback.Paused, Position: 2 * time.Second, Rate: 1}
		frame, err := timeline.Frame(sample, 640, 360)
		if err != nil || frame.Stats.Loaded != 64 || frame.Stats.SkippedEmpty != 0 {
			t.Fatal("fixture load is incomplete", err)
		}
		found := false
		for _, item := range frame.Items {
			found = found || item.Index == 0 && item.Text == entries[0].Text
		}
		if !found {
			t.Fatal("native Chinese opening phrase is absent at the proof position")
		}
		sample.Position = 12500 * time.Millisecond
		frame, err = timeline.Frame(sample, 640, 360)
		if err != nil || frame.Stats.Overflow == 0 || len(frame.Items) == 0 || len(frame.Items) > danmaku.MaxLanes {
			t.Fatal("fixture cannot exercise bounded dense presentation", err)
		}
		timeline.Close()
	}
}
