package transport

import (
	"my-bilibili/internal/catalog"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type requestSpec struct {
	path   string
	params url.Values
	needs  preparationNeed
	limit  int
}

func routeRequest(route Route, input url.Values, json bool) (requestSpec, error) {
	r := requestSpec{params: make(url.Values), limit: jsonLimit}
	if json != (route >= Nav && route <= PlayURL) || route < Nav || route > DanmakuSegment {
		return r, catalog.Invalid
	}
	for key, values := range input {
		if len(values) != 1 {
			return r, catalog.Invalid
		}
		r.params[key] = []string{values[0]}
	}
	var allowed []string
	constants := map[string]string{}
	valid := true
	switch route {
	case Nav:
		r.path = "/x/web-interface/nav"
	case Search:
		r.path, r.needs = "/x/web-interface/wbi/search/type", wbiPreparation|browserPreparation
		allowed = []string{"keyword", "page", "search_type", "order"}
		constants["search_type"], constants["order"] = "video", "totalrank"
		valid = queryValue(r.params.Get("keyword")) && decimal(r.params.Get("page"), false, 31)
	case Detail:
		r.path = "/x/web-interface/view"
		allowed = []string{"bvid", "aid"}
		bv, aid := r.params.Get("bvid"), r.params.Get("aid")
		_, hasBV := r.params["bvid"]
		_, hasAV := r.params["aid"]
		valid = hasBV != hasAV && (hasBV && validBV(bv) || hasAV && decimal(aid, false, 63))
	case PlayURL:
		r.path, r.needs = "/x/player/wbi/playurl", wbiPreparation
		allowed = []string{"bvid", "cid", "fnver", "fnval", "qn"}
		constants["fnver"], constants["fnval"] = "0", "16"
		valid = validBV(r.params.Get("bvid")) && decimal(r.params.Get("cid"), false, 63) && decimal(r.params.Get("qn"), true, 31)
	case DanmakuView:
		r.path = "/x/v2/dm/web/view"
		allowed = []string{"type", "oid"}
		constants["type"] = "1"
		valid = decimal(r.params.Get("oid"), false, 63)
	case DanmakuSegment:
		r.path, r.limit = "/x/v2/dm/web/seg.so", segmentLimit
		allowed = []string{"type", "oid", "segment_index"}
		constants["type"] = "1"
		valid = decimal(r.params.Get("oid"), false, 63) && decimal(r.params.Get("segment_index"), false, 31)
	}
	if !valid {
		return r, catalog.Invalid
	}
	for key := range r.params {
		known := false
		for _, name := range allowed {
			if name == key {
				known = true
				break
			}
		}
		if !known {
			return r, catalog.Invalid
		}
	}
	for key, fixed := range constants {
		if values, ok := r.params[key]; ok && values[0] != fixed {
			return r, catalog.Invalid
		}
		r.params.Set(key, fixed)
	}
	return r, nil
}

func validBV(value string) bool {
	if len(value) != 12 || !strings.HasPrefix(value, "BV") {
		return false
	}
	for _, c := range []byte(value[2:]) {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}
func decimal(value string, zero bool, bits int) bool {
	if len(value) == 0 || len(value) > 19 {
		return false
	}
	for _, c := range []byte(value) {
		if c < '0' || c > '9' {
			return false
		}
	}
	v, err := strconv.ParseUint(value, 10, bits)
	return err == nil && (zero || v > 0)
}
func queryValue(value string) bool {
	if len(value) == 0 || len(value) > 512 || !utf8.ValidString(value) || utf8.RuneCountInString(value) > 128 || strings.TrimSpace(value) == "" {
		return false
	}
	for _, c := range value {
		if unicode.IsControl(c) {
			return false
		}
	}
	return true
}
