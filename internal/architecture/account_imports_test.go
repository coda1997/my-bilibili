package architecture

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAccountDependencyBoundary(t *testing.T) {
	for _, target := range []struct{ os, arch, cgo string }{{"darwin", "arm64", "0"}, {"darwin", "arm64", "1"}, {"linux", "amd64", "0"}, {"linux", "amd64", "1"}, {"windows", "amd64", "0"}, {"windows", "amd64", "1"}} {
		t.Run(target.os+"/cgo="+target.cgo, func(t *testing.T) {
			command := exec.Command("go", "list", "-deps", "-json", "./internal/account")
			command.Dir = filepath.Join("..", "..")
			command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off", "CGO_ENABLED="+target.cgo, "GOOS="+target.os, "GOARCH="+target.arch)
			output, err := command.Output()
			if err != nil {
				t.Fatal("account dependency inspection failed")
			}
			decoder := json.NewDecoder(bytes.NewReader(output))
			seen := false
			for {
				var pkg struct {
					ImportPath string
					Standard   bool
				}
				err := decoder.Decode(&pkg)
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal("dependency output invalid")
				}
				if pkg.ImportPath == "my-bilibili/internal/account" {
					seen = true
					continue
				}
				if !pkg.Standard {
					t.Errorf("account imports nonstandard dependency %q", pkg.ImportPath)
				}
				if pkg.ImportPath == "net/http" {
					t.Error("account imports HTTP transport")
				}
			}
			if !seen {
				t.Fatal("account root absent from inspection")
			}
		})
	}
}
