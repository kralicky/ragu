package ragu

import (
	"fmt"
	"go/build"
	"runtime"
	"strings"
)

// Returns the full package name of the calling function. If the calling
// function is in a test package, returns the non-test package name.
func CallingFuncPackage() (*build.Package, error) {
	pc, _, _, ok := runtime.Caller(2)
	if !ok {
		return nil, fmt.Errorf("could not get caller information")
	}
	fn := runtime.FuncForPC(pc).Name()
	index := strings.LastIndexByte(fn, '.')
	pkg := strings.TrimSuffix(fn[:index], "_test")
	return build.Default.Import(pkg, ".", build.FindOnly)
}

func apply[T any, R any](fn func(T) R) func(T, int) R {
	return func(t T, _ int) R {
		return fn(t)
	}
}
