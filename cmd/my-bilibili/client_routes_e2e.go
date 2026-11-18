//go:build e2e

package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"strconv"
	"sync"

	"my-bilibili/internal/catalog"
)

// No fallback transport exists: every response below is original synthetic
// protocol data, including covers, with the actual adapters as consumers.
type syntheticClientRoutes struct {
	scene      string
	gate       <-chan struct{}
	cover      []byte
	mu         sync.Mutex
	failedPage bool
}

func newSyntheticClientRoutes(scene string, gate <-chan struct{}) (*syntheticClientRoutes, error) {
	img := image.NewRGBA(image.Rect(0, 0, 32, 18))
	for y := 0; y < 18; y++ {
		for x := 0; x < 32; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(40 + x*4), G: uint8(80 + y*4), B: 160, A: 255})
		}
	}
	var data bytes.Buffer
	if png.Encode(&data, img) != nil {
		return nil, catalog.Unavailable
	}
	return &syntheticClientRoutes{scene: scene, gate: gate, cover: data.Bytes()}, nil
}
func syntheticResponse(data []byte) *http.Response {
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(data))}
}
func syntheticJSON(value any) (*http.Response, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, catalog.InvalidResponse
	}
	return syntheticResponse(data), nil
}
func syntheticEnvelope(data any) (*http.Response, error) {
	return syntheticJSON(map[string]any{"code": 0, "data": data})
}

func (s *syntheticClientRoutes) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil || req.URL == nil || req.Method != http.MethodGet || req.URL.Scheme != "https" {
		return nil, catalog.Invalid
	}
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	if req.URL.Host == "cover.example.test" {
		if req.Header.Get("Cookie") != "" || req.Header.Get("Authorization") != "" {
			return nil, catalog.Invalid
		}
		switch req.URL.Path {
		case "/1.png", "/2.png", "/3.png":
			return syntheticResponse(s.cover), nil
		}
		return nil, catalog.Invalid
	}
	if req.URL.Host == "www.bilibili.com" && req.URL.Path == "/" {
		if req.Header.Get("Cookie") != "" {
			return nil, catalog.Invalid
		}
		r := syntheticResponse([]byte("original synthetic homepage"))
		r.Header.Add("Set-Cookie", "buvid3=SYNTHETIC_CLIENT_BROWSER; Domain=.bilibili.com; Path=/; Secure; Max-Age=3600")
		return r, nil
	}
	if req.URL.Host != "api.bilibili.com" {
		return nil, catalog.Unsupported
	}
	q := req.URL.Query()
	switch req.URL.Path {
	case "/x/web-interface/nav":
		return syntheticEnvelope(map[string]any{"isLogin": true, "wbi_img": map[string]any{"img_url": "https://keys.example.test/0123456789abcdef0123456789abcdef.png", "sub_url": "https://keys.example.test/fedcba9876543210fedcba9876543210.png"}})
	case "/x/web-interface/wbi/search/type":
		query, page := q.Get("keyword"), q.Get("page")
		if query == "hold" && s.scene == "close-held" {
			<-s.gate
			return nil, catalog.Canceled
		}
		if query == "空结果" && page == "1" {
			return syntheticEnvelope(map[string]any{"page": 1, "numPages": 0, "numResults": 0, "result": []any{}})
		}
		if query != "测试视频" || page != "1" && page != "2" {
			return nil, catalog.Invalid
		}
		if s.scene == "search" && page == "2" {
			s.mu.Lock()
			fail := !s.failedPage
			s.failedPage = true
			s.mu.Unlock()
			if fail {
				r := syntheticResponse(nil)
				r.StatusCode = 503
				return r, nil
			}
		}
		first := 1
		if page == "2" {
			first = 2
		}
		rows := []any{}
		for n := first; n < first+2; n++ {
			rows = append(rows, map[string]any{"type": "video", "bvid": syntheticBV(n), "title": "原创测试视频" + strconv.Itoa(n), "author": "测试作者", "duration": "00:30", "pic": "https://cover.example.test/" + strconv.Itoa(n) + ".png"})
		}
		return syntheticEnvelope(map[string]any{"page": first, "numPages": 2, "numResults": 3, "result": rows})
	case "/x/web-interface/view":
		n := 0
		for i := 1; i <= 3; i++ {
			if q.Get("bvid") == syntheticBV(i) || q.Get("aid") == strconv.Itoa(i) {
				n = i
			}
		}
		if n == 0 {
			return nil, catalog.Invalid
		}
		duration := 30
		if s.scene == "segments" {
			duration = 720
		}
		return syntheticEnvelope(map[string]any{"bvid": syntheticBV(n), "aid": n, "cid": n*100 + 1, "title": "原创测试视频" + strconv.Itoa(n), "owner": map[string]any{"name": "测试作者"}, "desc": "中文弹幕与续播验证", "pic": "https://cover.example.test/" + strconv.Itoa(n) + ".png",
			"rights": map[string]any{"movie": 0, "pay": 0, "ugc_pay": 0, "is_stein_gate": 0, "ugc_pay_preview": 0, "arc_pay": 0}, "is_chargeable_season": false, "is_upower_exclusive": false, "is_upower_preview": false,
			"pages": []any{map[string]any{"cid": n*100 + 1, "page": 1, "part": "第一部分", "duration": duration}, map[string]any{"cid": n*100 + 2, "page": 2, "part": "第二部分", "duration": duration}}})
	case "/x/player/wbi/playurl":
		valid := false
		for i := 1; i <= 3; i++ {
			if q.Get("bvid") == syntheticBV(i) && (q.Get("cid") == strconv.Itoa(i*100+1) || q.Get("cid") == strconv.Itoa(i*100+2)) {
				valid = true
			}
		}
		if !valid || q.Get("qn") != "64" && q.Get("qn") != "80" || q.Get("fnval") != "16" || q.Get("fnver") != "0" {
			return nil, catalog.Invalid
		}
		video := func(id, w, h, bandwidth int) map[string]any {
			return map[string]any{"id": id, "width": w, "height": h, "bandwidth": bandwidth, "codecid": 7, "codecs": "avc1.640028", "mimeType": "video/mp4", "baseUrl": "https://video.example.test/quality-" + strconv.Itoa(id) + ".m4s"}
		}
		return syntheticEnvelope(map[string]any{"quality": 80, "accept_quality": []int{80, 64}, "dash": map[string]any{"video": []any{video(80, 1920, 1080, 5000000), video(64, 1280, 720, 2000000)}, "audio": []any{map[string]any{"id": 30280, "bandwidth": 192000, "codecs": "mp4a.40.2", "mimeType": "audio/mp4", "baseUrl": "https://audio.example.test/original.m4s"}}}})
	case "/x/v2/dm/web/view":
		if !syntheticCID(q.Get("oid")) {
			return nil, catalog.Invalid
		}
		return syntheticResponse([]byte{8, 0}), nil
	case "/x/v2/dm/web/seg.so":
		if !syntheticCID(q.Get("oid")) {
			return nil, catalog.Invalid
		}
		index := q.Get("segment_index")
		if index != "1" && !(index == "2" && s.scene == "segments") {
			return nil, catalog.Invalid
		}
		return syntheticResponse(syntheticSegment(index == "2")), nil
	}
	return nil, catalog.Unsupported
}
func syntheticBV(n int) string { return "BV000000000" + strconv.Itoa(n) }
func syntheticCID(value string) bool {
	switch value {
	case "101", "102", "201", "202", "301", "302":
		return true
	}
	return false
}
func syntheticSegment(second bool) []byte {
	times := []uint64{0, 1000, 4000, 5000, 18000}
	if second {
		times = []uint64{361000}
	}
	var data []byte
	for i, at := range times {
		id := uint64(i + 1)
		if second {
			id += 100
		}
		entry := binary.AppendUvarint([]byte{8}, id)
		entry = binary.AppendUvarint(append(entry, 16), at)
		entry = append(entry, 24, 1)
		entry = binary.AppendUvarint(append(entry, 40), 0xffffff)
		text := []byte("中文弹幕验证")
		entry = binary.AppendUvarint(append(entry, 58), uint64(len(text)))
		entry = append(entry, text...)
		data = binary.AppendUvarint(append(data, 10), uint64(len(entry)))
		data = append(data, entry...)
	}
	return data
}
