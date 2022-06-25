package ragu

import (
	"bytes"
	"fmt"
	"go/build"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jhump/protoreflect/desc/protoparse"
	"golang.org/x/mod/module"
)

type Source interface {
	Read() (io.ReadCloser, error)
	AbsolutePath() string
}

type protobufFile struct {
	path string
}

func (s *protobufFile) Read() (io.ReadCloser, error) {
	return os.Open(s.path)
}

func (s *protobufFile) AbsolutePath() string {
	return s.path
}

func FromFile(path string) Source {
	if filepath.IsAbs(path) {
		return &protobufFile{path: path}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		panic(fmt.Errorf("could not convert %s to an absolute path: %w", path, err))
	}
	return &protobufFile{
		path: abs,
	}
}

type protobufData struct {
	contents []byte
	filename string
}

func (s *protobufData) Read() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.contents)), nil
}

func (s *protobufData) AbsolutePath() string {
	return s.filename
}

func FromData(data []byte, filename string) Source {
	return &protobufData{
		contents: data,
		filename: filename,
	}
}

func SourceAccessor(pkg *build.Package, sources ...Source) protoparse.FileAccessor {
	sm := map[string]func() (io.ReadCloser, error){}
	for _, src := range sources {
		src := src
		sm[src.AbsolutePath()] = src.Read
	}
	return func(filename string) (io.ReadCloser, error) {
		// one of the provided source files
		if fn, ok := sm[filename]; ok {
			return fn()
		}

		// skip well-known files, these are handled by protoparse
		if strings.HasPrefix(filename, "google/") {
			return nil, os.ErrNotExist
		}

		// a file named by the caller's go module
		if strings.HasPrefix(filename, pkg.ImportPath) {
			return os.Open(filepath.Join(pkg.Dir, filename[len(pkg.ImportPath)+1:]))
		}

		if rc, err := readFromModuleCache(filename); err == nil {
			return rc, nil
		}
		return nil, os.ErrNotExist
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
