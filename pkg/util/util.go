package util

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

func Map[T any, R any](collection []T, iteratee func(T) R) []R {
	result := make([]R, len(collection))

	for i, item := range collection {
		result[i] = iteratee(item)
	}

	return result
}
