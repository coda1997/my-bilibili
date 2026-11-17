// Package previewbundle owns the developer preview's packaging and runtime boundary.
package previewbundle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"my-bilibili/internal/fixture"
)

const Library = "/Users/dadachen/workspace/my-bilibili/.agent/runtime/mpv-0.42.0/m2-rebuild-r1/prefix/lib/libmpv.2.dylib"
const RuntimeManifest = "/Users/dadachen/workspace/my-bilibili/.agent/runtime/mpv-0.42.0/m2-rebuild-r1/runtime-manifest.json"
const RuntimeSHA256 = "a5fb151f6c41d6c0eb12f214ec187581b0589dc57f37911887cf133eda99fb54"
const LibrarySHA256 = "e92b86a0c9a239d05ef0127081007d1bc7cca56b54337388302ed9c83a5532da"

var assetNames = []string{"manifest.json", "metadata.json", "part-1.mp4", "part-2.mp4", "cover.png"}

// Resources accepts a bundle executable, never the process working directory.
func Resources(executable string) (string, error) {
	if !filepath.IsAbs(executable) || filepath.Base(filepath.Dir(executable)) != "MacOS" || filepath.Base(filepath.Dir(filepath.Dir(executable))) != "Contents" {
		return "", errors.New("preview must run from its developer app bundle")
	}
	return filepath.Join(filepath.Dir(filepath.Dir(executable)), "Resources", "fixtures"), nil
}

func Hash(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > 256<<20 {
		return "", errors.New("artifact unavailable or invalid")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", errors.New("artifact unavailable")
	}
	h := sha256.New()
	_, err = io.Copy(h, f)
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return "", errors.New("artifact read failed")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyRuntime pins the manifest and every dependency, including libmpv. The
// normal application has no runtime selection flag, environment or UI setting.
func VerifyRuntime() error { return verifyRuntime(RuntimeManifest) }
func verifyRuntime(path string) error {
	h, err := Hash(path)
	if err != nil || h != RuntimeSHA256 {
		return errors.New("verified developer playback runtime is unavailable; rebuild the assigned runtime prerequisite")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return errors.New("runtime manifest unavailable")
	}
	type artifact struct{ Path, SHA256 string }
	var m struct {
		SourceCommit, API string
		Library           artifact
		Dependencies      []artifact
	}
	if json.Unmarshal(data, &m) != nil || m.API != "2.6" || m.SourceCommit != "b0841e8c4d156b8403dc1876c4cbd9011eb583f3" || m.Library.Path != Library || m.Library.SHA256 != LibrarySHA256 || len(m.Dependencies) != 29 {
		return errors.New("runtime manifest invalid")
	}
	for _, a := range append(m.Dependencies, m.Library) {
		h, err := Hash(a.Path)
		if !filepath.IsAbs(a.Path) || err != nil || h != a.SHA256 {
			return errors.New("verified developer runtime artifact changed")
		}
	}
	return nil
}

// Populate creates only a fresh bundle. It never replaces an existing output.
// The caller owns cleanup if a later build fails.
func Populate(output, assets string) error {
	if !filepath.IsAbs(output) || filepath.Ext(output) != ".app" {
		return errors.New("output must be an absolute fresh .app path")
	}
	if _, err := fixture.New(assets); err != nil {
		return errors.New("fixture resources failed integrity validation")
	}
	if err := os.Mkdir(output, 0755); err != nil {
		return errors.New("output app already exists or parent is unavailable")
	}
	for _, dir := range []string{"MacOS", "Resources/fixtures"} {
		if err := os.MkdirAll(filepath.Join(output, "Contents", dir), 0755); err != nil {
			return err
		}
	}
	for _, name := range assetNames {
		b, err := os.ReadFile(filepath.Join(assets, name))
		if err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(output, "Contents", "Resources", "fixtures", name), b, 0644); err != nil {
			return err
		}
	}
	if _, err := fixture.New(filepath.Join(output, "Contents", "Resources", "fixtures")); err != nil {
		return errors.New("copied fixture integrity failed")
	}
	return os.WriteFile(filepath.Join(output, "Contents", "Info.plist"), []byte(plist), 0644)
}

const plist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>my-bilibili</string>
<key>CFBundleIdentifier</key><string>local.my-bilibili.preview</string>
<key>CFBundleName</key><string>My Bilibili</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleShortVersionString</key><string>0.0.3</string>
<key>NSHighResolutionCapable</key><true/>
</dict></plist>
`
