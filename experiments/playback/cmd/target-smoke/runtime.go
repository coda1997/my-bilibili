package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const runtimeDigest = "a633103c161ee4b648722ba041c695825c5532170bde7311f6ceb917fc05e2bc"
const libraryDigest = "8f7c25d37eab7245731fa46734d2003601b272d2dcade2fbb925157e55246bbd"

type artifact struct {
	Path, SHA256 string
	Bytes        int64
}
type runtimeManifest struct {
	SourceCommit string
	API          string
	Library      artifact
	Dependencies []artifact
}

func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func artifactHash(path string, limit int64) (string, int64, error) {
	info, err := os.Lstat(path)
	if !filepath.IsAbs(path) || err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > limit {
		return "", 0, errors.New("invalid artifact")
	}
	b, err := os.ReadFile(path)
	if err != nil || int64(len(b)) != info.Size() {
		return "", 0, errors.New("artifact unavailable")
	}
	return digest(b), info.Size(), nil
}
func verifyArtifact(a artifact) error {
	h, n, err := artifactHash(a.Path, 128<<20)
	if err != nil || h != a.SHA256 || n != a.Bytes {
		return errors.New("runtime artifact changed")
	}
	return nil
}
func verifyRuntime(path string) (runtimeManifest, string, error) {
	var m runtimeManifest
	h, _, err := artifactHash(path, 4<<20)
	if err != nil || h != runtimeDigest {
		return m, "", errors.New("runtime manifest identity mismatch")
	}
	b, err := os.ReadFile(path)
	if err != nil || digest(b) != h || json.Unmarshal(b, &m) != nil || m.SourceCommit != "b0841e8c4d156b8403dc1876c4cbd9011eb583f3" || m.API != "2.6" || m.Library.SHA256 != libraryDigest || len(m.Dependencies) == 0 || len(m.Dependencies) > 256 {
		return m, "", errors.New("invalid pinned runtime")
	}
	if err = verifyArtifact(m.Library); err != nil {
		return m, "", err
	}
	for _, a := range m.Dependencies {
		if err = verifyArtifact(a); err != nil {
			return m, "", err
		}
	}
	return m, h, nil
}
