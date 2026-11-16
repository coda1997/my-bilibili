package previewbundle

import (
	"os"
	"path/filepath"
	"testing"

	"my-bilibili/internal/fixture"
)

func TestBundleResourcesOutsideWorkingDirectory(t *testing.T) {
	assets, err := filepath.Abs("../fixture/assets")
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "Preview.app")
	if err := Populate(output, assets); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(output, "Contents", "MacOS", "my-bilibili")
	resources, err := Resources(executable)
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	source, err := fixture.New(resources)
	if err != nil {
		t.Fatal(err)
	}
	video, err := source.Detail()
	if err != nil || video.CoverID != "preview-cover" {
		t.Fatal(video, err)
	}
	for _, part := range video.Parts {
		in, err := source.Resolve(part.ID)
		if err != nil || !filepath.IsAbs(in.FilePath) {
			t.Fatal(in, err)
		}
	}
	if _, err := os.ReadFile(filepath.Join(resources, "cover.png")); err != nil {
		t.Fatal(err)
	}
	if err := Populate(output, assets); err == nil {
		t.Fatal("overwrote an existing bundle")
	}
	if err := os.WriteFile(filepath.Join(resources, "part-1.mp4"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.New(resources); err == nil {
		t.Fatal("accepted changed bundle asset")
	}
}
func TestInvalidBundleAndRuntimeBoundaries(t *testing.T) {
	for _, path := range []string{"", "my-bilibili", "/tmp/my-bilibili", "/tmp/x.app/Resources/my-bilibili"} {
		if _, err := Resources(path); err == nil {
			t.Fatalf("accepted %q", path)
		}
	}
	if err := Populate("relative.app", "../fixture/assets"); err == nil {
		t.Fatal("relative output accepted")
	}
	if err := Populate(filepath.Join(t.TempDir(), "Bad.app"), t.TempDir()); err == nil {
		t.Fatal("missing fixtures accepted")
	}
	manifest := filepath.Join(t.TempDir(), "runtime.json")
	if err := verifyRuntime(manifest); err == nil {
		t.Fatal("missing runtime accepted")
	}
	if err := os.WriteFile(manifest, []byte(`{"Library":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyRuntime(manifest); err == nil {
		t.Fatal("unverified runtime accepted")
	}
	if _, err := Hash(filepath.Dir(manifest)); err == nil {
		t.Fatal("directory accepted as artifact")
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(manifest, link); err == nil {
		if _, err := Hash(link); err == nil {
			t.Fatal("symlink accepted as artifact")
		}
	}
}
