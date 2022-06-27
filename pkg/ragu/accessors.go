package ragu

import (
	"fmt"
	"go/build"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/mod/module"
)

func SourceAccessor(localPkg *build.Package) func(filename string) (io.ReadCloser, error) {
	return func(filename string) (io.ReadCloser, error) {
		if strings.HasPrefix(filename, localPkg.ImportPath) {
			// local to this package
			localPath := path.Join(localPkg.Dir, strings.TrimPrefix(filename, localPkg.ImportPath))
			return os.Open(localPath)
		}

		if f, err := os.Open(filename); err == nil {
			return f, nil
		}

		rc, err := readFromModuleCache(filename)
		if err != nil {
			return nil, fmt.Errorf("could not find %s locally or in go module cache: %w", filename, err)
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
		cmd.Env = append(os.Environ(),
			"GOOS="+runtime.GOOS,
			"GOARCH="+runtime.GOARCH,
			"GOROOT="+runtime.GOROOT(),
		)
		if out, err := cmd.Output(); err == nil {
			protoFilePath = filepath.Join(strings.TrimSpace(string(out)), filename)
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
