package architecture

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Inspect the complete dependency graph, so a business package cannot hide a
// native/UI/transport dependency behind a helper. Evaluate each product/CI
// target's build tags with and without cgo. go list inspects imports without
// compiling native code or claiming to execute on those operating systems.
func TestBusinessDependencyBoundary(t *testing.T) {
	for _, target := range []struct{ goos, goarch, cgo string }{
		{"darwin", "arm64", "0"}, {"darwin", "arm64", "1"},
		{"linux", "amd64", "0"}, {"linux", "amd64", "1"},
		{"windows", "amd64", "0"}, {"windows", "amd64", "1"},
	} {
		t.Run(target.goos+"/"+target.goarch+"/cgo="+target.cgo, func(t *testing.T) {
			cmd := exec.Command("go", "list", "-deps", "-json", "./internal/catalog", "./internal/app")
			cmd.Dir = filepath.Join("..", "..")
			cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off", "CGO_ENABLED="+target.cgo, "GOOS="+target.goos, "GOARCH="+target.goarch)
			output, err := cmd.Output()
			if err != nil {
				var exit *exec.ExitError
				if errors.As(err, &exit) {
					t.Fatalf("inspect business dependencies: %v\n%s", err, exit.Stderr)
				}
				t.Fatal(err)
			}
			decoder := json.NewDecoder(bytes.NewReader(output))
			seen := make(map[string]bool)
			for {
				var pkg struct {
					ImportPath string
					Standard   bool
				}
				err := decoder.Decode(&pkg)
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Fatalf("decode business dependency: %v", err)
				}
				seen[pkg.ImportPath] = true
				if pkg.Standard {
					continue
				}
				switch pkg.ImportPath {
				case "my-bilibili/internal/catalog", "my-bilibili/internal/app", "my-bilibili/experiments/playback/contract":
				default:
					t.Errorf("business dependency %q is outside the reviewed plain catalog/app/playback boundary", pkg.ImportPath)
				}
			}
			for _, root := range []string{"my-bilibili/internal/catalog", "my-bilibili/internal/app"} {
				if !seen[root] {
					t.Errorf("dependency output omitted required package %q", root)
				}
			}
		})
	}
}
