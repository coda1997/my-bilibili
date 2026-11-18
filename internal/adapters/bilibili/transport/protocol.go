package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"my-bilibili/internal/catalog"
	"unicode/utf8"
)

const jsonDepthLimit = 32

type envelope struct {
	code    int64
	data    map[string]json.RawMessage
	voucher bool
}

// Validate structural bounds and duplicate keys before typed decoding, so no
// critical field can be selected differently by two feature consumers.
func validJSON(body []byte) bool {
	if !utf8.Valid(body) {
		return false
	}
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	var value func(int) bool
	value = func(depth int) bool {
		if depth > jsonDepthLimit {
			return false
		}
		token, err := d.Token()
		if err != nil {
			return false
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return true
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return false
				}
				name, ok := key.(string)
				if !ok || seen[name] {
					return false
				}
				seen[name] = true
				if !value(depth + 1) {
					return false
				}
			}
			end, err := d.Token()
			return err == nil && end == json.Delim('}')
		case '[':
			for d.More() {
				if !value(depth + 1) {
					return false
				}
			}
			end, err := d.Token()
			return err == nil && end == json.Delim(']')
		default:
			return false
		}
	}
	if !value(0) {
		return false
	}
	_, err := d.Token()
	return err == io.EOF
}
func object(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return nil, false
	}
	return fields, true
}
func text(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", false
	}
	var value string
	err := json.Unmarshal(raw, &value)
	return value, err == nil
}

func decodeEnvelope(body []byte) (envelope, error) {
	var result envelope
	if !validJSON(body) {
		return result, catalog.InvalidResponse
	}
	fields, ok := object(body)
	if !ok {
		return result, catalog.InvalidResponse
	}
	raw := fields["code"]
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &result.code) != nil {
		return result, catalog.InvalidResponse
	}
	if raw, exists := fields["data"]; exists && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		if result.data, ok = object(raw); !ok {
			return result, catalog.InvalidResponse
		}
	}
	if result.code == 0 && result.data == nil {
		return result, catalog.InvalidResponse
	}
	for _, fields := range []map[string]json.RawMessage{fields, result.data} {
		if raw, exists := fields["v_voucher"]; exists {
			value, ok := text(raw)
			if !ok {
				return result, catalog.InvalidResponse
			}
			result.voucher = result.voucher || value != ""
		}
	}
	return result, nil
}
func serviceCode(code int64) catalog.Code {
	switch code {
	case 0:
		return catalog.None
	case -101:
		return catalog.LoginRequired
	case -412:
		return catalog.VerificationRequired
	case -403, 62004, 62012:
		return catalog.Restricted
	case -404, 62002:
		return catalog.Unavailable
	default:
		return catalog.ServiceUnavailable
	}
}

func (s *Scope) acceptJSON(ctx context.Context, route Route, body []byte) (envelope, error) {
	value, err := decodeEnvelope(body)
	if err != nil {
		return value, err
	}
	if err := s.check(ctx); err != nil {
		return value, err
	}
	if value.voucher {
		return value, catalog.VerificationRequired
	}
	if route != Nav {
		if code := serviceCode(value.code); code != catalog.None {
			return value, code
		}
		return value, nil
	}
	isLogin := false
	if raw, exists := value.data["isLogin"]; exists {
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &isLogin) != nil {
			return value, catalog.InvalidResponse
		}
	} else if value.code == 0 {
		return value, catalog.InvalidResponse
	}
	if value.code != 0 && value.code != -101 {
		return value, serviceCode(value.code)
	}
	if value.code == -101 && isLogin {
		return value, catalog.InvalidResponse
	}
	s.client.mu.Lock()
	defer s.client.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return value, contextCode(err)
	}
	if err := s.live(); err != nil {
		return value, err
	}
	if (value.code == -101 || !isLogin) && s.epoch > 0 {
		s.expiredEpoch = s.epoch
		s.client.replace(0, nil, s.epoch)
		return value, catalog.SessionExpired
	}
	return value, nil
}

func (s *Scope) acceptProtobuf(ctx context.Context, body []byte) error {
	if err := s.check(ctx); err != nil {
		return err
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil
	}
	switch trimmed[0] {
	case '<', '[':
		return catalog.InvalidResponse
	case '{':
		value, err := decodeEnvelope(body)
		if err != nil {
			return err
		}
		if value.voucher {
			return catalog.VerificationRequired
		}
		if code := serviceCode(value.code); code != catalog.None {
			return code
		}
		return catalog.InvalidResponse
	}
	return nil
}
