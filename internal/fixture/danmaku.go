package fixture

import (
	"strconv"
	"time"

	"my-bilibili/internal/catalog"
	"my-bilibili/internal/danmaku"
)

// Danmaku returns original, independently owned text for the fixed local parts.
// It performs no I/O and is separate from the media Source contract.
func Danmaku(part catalog.PartID) ([]danmaku.Entry, error) {
	title, prefix := "", ""
	switch part {
	case "part-1":
		title, prefix = "中文弹幕测试", "第一分P"
	case "part-2":
		title, prefix = "第二分P弹幕", "第二分P"
	default:
		return nil, danmaku.Invalid
	}
	entries := []danmaku.Entry{
		{At: 0, Mode: danmaku.FixedTop, Text: title, Color: 0xffffff},
		{At: time.Second, Mode: danmaku.ScrollRTL, Text: "暂停后位置不变", Color: 0xffb7d5},
		{At: 1500 * time.Millisecond, Mode: danmaku.ScrollLTR, Text: prefix + " · 向右飞过的中文", Color: 0x80d4ff},
		{At: 1750 * time.Millisecond, Mode: danmaku.FixedBottom, Text: prefix + " · 音画同行，一起看", Color: 0xffffff},
		{At: 4500 * time.Millisecond, Mode: danmaku.FixedTop, Text: "拖动后重新出现", Color: 0xffb7d5},
		{At: 4600 * time.Millisecond, Mode: danmaku.ScrollRTL, Text: prefix + " · 跟随视频进度", Color: 0xffffff},
		{At: 5 * time.Second, Mode: danmaku.FixedBottom, Text: prefix + " · 支持全屏播放", Color: 0x80d4ff},
		{At: 9 * time.Second, Mode: danmaku.ScrollRTL, Text: prefix + " · 再来一条测试弹幕", Color: 0xffffff},
	}
	modes := [...]danmaku.Mode{danmaku.ScrollRTL, danmaku.FixedBottom, danmaku.FixedTop, danmaku.ScrollLTR}
	colors := [...]uint32{0xffffff, 0xffb7d5, 0x80d4ff}
	for i := 0; i < 56; i++ {
		entries = append(entries, danmaku.Entry{
			At:   12*time.Second + time.Duration(i%4)*time.Millisecond,
			Mode: modes[i%len(modes)], Text: prefix + " · 密集弹幕 " + strconv.Itoa(i+1),
			Color: colors[i%len(colors)],
		})
	}
	return entries, nil
}
