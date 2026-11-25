package architecture

import (
	"strings"
	"testing"
)

func TestAccountDependencyBoundary(t *testing.T) {
	allowed := map[string]bool{
		"my-bilibili/internal/account": true,
	}
	for _, target := range ciBuildTargets {
		t.Run(target.goos+"/"+target.goarch+"/cgo="+target.cgo, func(t *testing.T) {
			pkgs := listPackagesJSON(t, target, "list", "-test", "-deps", "-json", "./internal/account")
			seen := make(map[string]bool, len(pkgs))
			for _, pkg := range pkgs {
				seen[pkg.ImportPath] = true
				base := baseImportPath(pkg.ImportPath)
				if base == "net/http" || strings.HasPrefix(base, "net/http/") {
					t.Errorf("account imports HTTP transport dependency %q", pkg.ImportPath)
				}
				if pkg.Standard {
					continue
				}
				if !isAllowedSyntheticTestPath(pkg.ImportPath, allowed) {
					t.Errorf("account imports nonstandard dependency %q", pkg.ImportPath)
				}
			}
			if !seen["my-bilibili/internal/account"] {
				t.Fatal("account root absent from inspection")
			}
			if !seen["my-bilibili/internal/account.test"] {
				t.Fatal("account test package absent from inspection")
			}
		})
	}
}
