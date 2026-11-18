package catalog

import (
	"strings"
	"testing"
)

func TestEntryCanonicalIdentityAndTrackingDiscard(t *testing.T) {
	for _, test := range []struct {
		input string
		want  Entry
	}{
		{"BV0000000001", Entry{BVEntry, "BV0000000001", 0}},
		{" \nBV0aZ1234567\t", Entry{BVEntry, "BV0aZ1234567", 0}},
		{"av00001", Entry{AVEntry, "1", 0}},
		{"av9223372036854775807", Entry{AVEntry, "9223372036854775807", 0}},
		{"https://www.bilibili.com/video/BV0000000001/?p=2&from=tracking&from=another", Entry{BVEntry, "BV0000000001", 2}},
		{"https://www.bilibili.com:443/video/av42?p=1000", Entry{AVEntry, "42", 1000}},
		{"HTTPS://WWW.BILIBILI.COM/video/BV0000000001?p=%31", Entry{BVEntry, "BV0000000001", 1}},
	} {
		got, err := ParseEntry(test.input)
		if err != nil || got != test.want {
			t.Fatalf("entry=%+v err=%v want=%+v", got, err, test.want)
		}
	}
}

func TestEntryRejectsMalformedAndUnsupportedInputs(t *testing.T) {
	for _, input := range []string{
		"", "word BV0000000001", "bv0000000001", "AV1", "av0", "av-1", "av+1",
		"av9223372036854775808", "av00000000000000000001", "BV000000000!", "BV00000000010",
		"BV00000\n00001", string([]byte{0xff}), strings.Repeat(" ", 2049) + "BV0000000001",
		"https://www.bilibili.com/video/%42V0000000001", "https://www.bilibili.com/video/BV0000000001/extra",
		"https://www.bilibili.com/video/BV0000000001//", "https://user@www.bilibili.com/video/BV0000000001",
		"https://www.bilibili.com:80/video/BV0000000001", "https://www.bilibili.com:/video/BV0000000001",
		"https://www.bilibili.com/video/BV0000000001#", "https://www.bilibili.com/video/BV0000000001?p=0",
		"https://www.bilibili.com/video/BV0000000001?p=-1", "https://www.bilibili.com/video/BV0000000001?p=1001",
		"https://www.bilibili.com/video/BV0000000001?p=1&p=2", "https://www.bilibili.com/video/BV0000000001?p=",
		"https://www.bilibili.com/video/BV0000000001?p=x", "https://www.bilibili.com/video/BV0000000001?track=%zz",
		"https://www.bilibili.com/video/BV0000000001?track=a;b",
		"https://www.bilibili.com/video/BV0000000001?track=\xffx",
		"https://www.bilibili.com/video/BV0000000001?track=\u0085x",
	} {
		if _, err := ParseEntry(input); err != Invalid {
			t.Fatalf("invalid input accepted/category=%v for %q", err, input)
		}
	}
	for _, input := range []string{"https://b23.tv/abc", "https://other.test/?url=BV0000000001", "http://www.bilibili.com/video/BV0000000001", "https://www.bilibili.com/bangumi/play/ep1", "https://www.bilibili.com./video/BV0000000001"} {
		if _, err := ParseEntry(input); err != Unsupported {
			t.Fatal("unsupported URL reinterpreted", err)
		}
	}
}

func TestQueryScalarByteAndControlBounds(t *testing.T) {
	for _, input := range []string{"video", " 中文 搜索 ", strings.Repeat("a", 128), strings.Repeat("界", 128), strings.Repeat("😀", 128)} {
		got, err := ValidateQuery(input)
		if err != nil || got != strings.TrimSpace(input) {
			t.Fatal("valid query rejected", err)
		}
	}
	for _, input := range []string{"", "  ", "a\nb", "a\tb", "x\x00", strings.Repeat("a", 129), strings.Repeat("😀", 129), string([]byte{0xff})} {
		if _, err := ValidateQuery(input); err != Invalid {
			t.Fatal("invalid query accepted", err)
		}
	}
}
