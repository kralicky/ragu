package ragu

import (
	"fmt"
	"go/build"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/mod/module"
)

func SourceAccessor(sourcePackages map[string]string) func(filename string) (io.ReadCloser, error) {
	return func(importName string) (io.ReadCloser, error) {
		if filename, ok := sourcePackages[importName]; ok {
			return os.Open(filename)
		}

		if f, err := os.Open(importName); err == nil {
			return f, nil
		}

		if strings.HasPrefix(importName, "google/") {
			return nil, os.ErrNotExist
		}

		if strings.HasPrefix(importName, "gogoproto/") {
			importName = "github.com/gogo/protobuf/" + importName
		}

		rc, err := readFromModuleCache(importName)
		if err != nil {
			return nil, fmt.Errorf("could not find %s locally or in go module cache: %w", importName, err)
		}
		return rc, nil
	}
}

func readFromModuleCache(dep string) (io.ReadCloser, error) {
	last := strings.LastIndex(dep, "/")
	if last == -1 {
		return nil, os.ErrNotExist
	}
	filename := dep[last+1:]
	if !strings.HasSuffix(filename, ".proto") {
		return nil, os.ErrNotExist
	}

	// check if the path (excluding the filename) is a well-formed go module
	modulePath := dep[:last]
	if err := module.CheckImportPath(modulePath); err != nil {
		return nil, os.ErrNotExist
	}

	// check module cache for the file
	var protoFilePath string
	pkg, err := build.Default.Import(modulePath, "", 0)
	if err != nil {
		cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", modulePath)
		env := append(os.Environ(),
			"GOOS="+runtime.GOOS,
			"GOARCH="+runtime.GOARCH,
			"GOROOT="+runtime.GOROOT(),
		)
		cmd.Env = env
		if out, err := cmd.Output(); err == nil {
			protoFilePath = filepath.Join(strings.TrimSpace(string(out)), filename)
		} else {
			// last resort: if the path contains more than 3 parts, see if we can walk the
			// module path up until we reach a path that exists. This has a decent chance
			// of being able to find proto files in subdirectories that don't contain
			// any go code.
			parts := strings.Split(modulePath, "/")
			if len(parts) > 3 {
				for i := len(parts) - 1; i >= 3; i-- {
					cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", strings.Join(parts[:i], "/"))
					cmd.Env = env
					if out, err := cmd.Output(); err == nil {
						// now add the parts that we skipped back to the end of the path
						protoFilePath = filepath.Join(strings.TrimSpace(string(out)), strings.Join(parts[i:], "/"), filename)
						break
					}
				}
			}
		}
	} else {
		protoFilePath = filepath.Join(pkg.Dir, filename)
	}
	// Check if proto file exists
	if _, err := os.Stat(protoFilePath); err != nil {
		return nil, os.ErrNotExist
	}
	return os.Open(protoFilePath)
}
