package architecture

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type buildTarget struct {
	goos, goarch, cgo string
}

var ciBuildTargets = []buildTarget{
	{"darwin", "arm64", "0"}, {"darwin", "arm64", "1"},
	{"linux", "amd64", "0"}, {"linux", "amd64", "1"},
	{"windows", "amd64", "0"}, {"windows", "amd64", "1"},
}

type listedPackage struct {
	ImportPath string
	Standard   bool
}

func baseImportPath(importPath string) string {
	base, _, _ := strings.Cut(importPath, " [")
	return base
}

// isAllowedSyntheticTestPath permits only <p>, <p>.test, <p> [<p>.test], and
// <p>_test [<p>.test] for packages explicitly listed in allowed. It never
// exempts arbitrary ForTest packages.
func isAllowedSyntheticTestPath(importPath string, allowed map[string]bool) bool {
	if allowed[importPath] {
		return true
	}
	if base, ok := strings.CutSuffix(importPath, ".test"); ok && allowed[base] {
		return true
	}
	base, rest, ok := strings.Cut(importPath, " [")
	if !ok || !strings.HasSuffix(rest, ".test]") {
		return false
	}
	owner := strings.TrimSuffix(rest, ".test]")
	if !allowed[owner] {
		return false
	}
	return base == owner || base == owner+"_test"
}

func TestAllowedSyntheticTestPathAntiVacuity(t *testing.T) {
	allowed := map[string]bool{
		"my-bilibili/internal/catalog": true,
	}
	for _, valid := range []string{
		"my-bilibili/internal/catalog",
		"my-bilibili/internal/catalog.test",
		"my-bilibili/internal/catalog [my-bilibili/internal/catalog.test]",
		"my-bilibili/internal/catalog_test [my-bilibili/internal/catalog.test]",
	} {
		if !isAllowedSyntheticTestPath(valid, allowed) {
			t.Errorf("rejected allowed synthetic test path %q", valid)
		}
	}
	for _, forbidden := range []string{
		"my-bilibili/internal/desktop",
		"my-bilibili/internal/desktop.test",
		"my-bilibili/internal/desktop [my-bilibili/internal/catalog.test]",
		"github.com/example/pkg [my-bilibili/internal/catalog.test]",
		"my-bilibili/internal/catalog [my-bilibili/internal/other.test]",
	} {
		if isAllowedSyntheticTestPath(forbidden, allowed) {
			t.Errorf("accepted forbidden dependency %q", forbidden)
		}
	}
}

func listPackagesJSON(t *testing.T, target buildTarget, args ...string) []listedPackage {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = filepath.Join("..", "..")
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off", "CGO_ENABLED="+target.cgo, "GOOS="+target.goos, "GOARCH="+target.goarch)
	output, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			t.Fatalf("go %v failed: %v\n%s", args, err, exit.Stderr)
		}
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(output))
	var pkgs []listedPackage
	for {
		var pkg listedPackage
		err := decoder.Decode(&pkg)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("decode go list output: %v", err)
		}
		pkgs = append(pkgs, pkg)
	}
	return pkgs
}

// Inspect the complete dependency graph (including test files), so a business
// package cannot hide a native/UI/transport dependency behind a helper or test.
// Evaluate each CI target's build tags with and without cgo.
func TestBusinessDependencyBoundary(t *testing.T) {
	roots := []string{
		"my-bilibili/internal/catalog",
		"my-bilibili/internal/app",
		"my-bilibili/internal/playback",
		"my-bilibili/internal/danmaku",
		"my-bilibili/internal/history",
	}
	allowed := make(map[string]bool, len(roots))
	for _, root := range roots {
		allowed[root] = true
	}
	for _, target := range ciBuildTargets {
		t.Run(target.goos+"/"+target.goarch+"/cgo="+target.cgo, func(t *testing.T) {
			pkgs := listPackagesJSON(t, target,
				"list", "-test", "-deps", "-json",
				"./internal/catalog", "./internal/app", "./internal/playback", "./internal/danmaku", "./internal/history",
			)
			seen := make(map[string]bool, len(pkgs))
			for _, pkg := range pkgs {
				seen[pkg.ImportPath] = true
				if pkg.Standard {
					continue
				}
				if !isAllowedSyntheticTestPath(pkg.ImportPath, allowed) {
					t.Errorf("business dependency %q is outside the reviewed plain business boundary", pkg.ImportPath)
				}
			}
			for _, root := range roots {
				if !seen[root] {
					t.Errorf("dependency output omitted required package %q", root)
				}
				if !seen[root+".test"] {
					t.Errorf("dependency output omitted required test package %q", root+".test")
				}
			}
		})
	}
}

// Product packages (including their tests) must not depend on experiments on
// any CI target or build tag. Probes may import product packages, never the reverse.
func TestProductDoesNotImportExperiments(t *testing.T) {
	for _, target := range ciBuildTargets {
		for _, tags := range []string{"", "e2e"} {
			name := target.goos + "/" + target.goarch + "/cgo=" + target.cgo + "/tags=" + tags
			t.Run(name, func(t *testing.T) {
				pkgs := listPackagesJSON(t, target,
					"list", "-test", "-deps", "-json", "-tags", tags,
					"./cmd/my-bilibili", "./internal/...",
				)
				seenMPV := false
				seenMainTest := false
				for _, pkg := range pkgs {
					base := baseImportPath(pkg.ImportPath)
					if base == "my-bilibili/internal/adapters/mpv" {
						seenMPV = true
					}
					if pkg.ImportPath == "my-bilibili/cmd/my-bilibili.test" {
						seenMainTest = true
					}
					if strings.HasPrefix(base, "my-bilibili/experiments/") {
						t.Errorf("product dependency %q is under experiments/", pkg.ImportPath)
					}
				}
				if !seenMainTest {
					t.Error("dependency output omitted my-bilibili/cmd/my-bilibili.test")
				}
				if target.goos == "darwin" && target.goarch == "arm64" && target.cgo == "1" && !seenMPV {
					t.Error("dependency output omitted the libmpv adapter used by cmd/my-bilibili")
				}
			})
		}
	}
}
