package account

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestOpaqueMaterialCopiesAndRedacts(t *testing.T) {
	const sentinel = "PRIVATE_ACCOUNT_SENTINEL"
	source := []byte(sentinel)
	material := NewMaterial(source)
	source[0] = 'x'
	returned := material.Bytes()
	returned[0] = 'y'
	if !bytes.Equal(material.Bytes(), []byte(sentinel)) || material.Empty() || !(Material{}).Empty() {
		t.Fatal("material ownership changed")
	}
	qr := NewQR(sentinel)
	if qr.Text() != sentinel || qr.Empty() || !(QR{}).Empty() {
		t.Fatal("QR presentation changed")
	}
	for _, value := range []any{material, qr, PollResult{Status: PollComplete, Material: material}, State{Phase: Waiting, QR: qr}} {
		for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
			if strings.Contains(fmt.Sprintf(verb, value), sentinel) {
				t.Fatal("private value formatted")
			}
		}
		encoded, err := json.Marshal(value)
		if err != nil || bytes.Contains(encoded, []byte(sentinel)) {
			t.Fatal("private value serialized")
		}
	}
}
