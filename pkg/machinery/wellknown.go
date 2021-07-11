package machinery

import (
	"bytes"
	"embed"
	"io"
)

//go:embed google
var WellKnownTypesFS embed.FS

func ReadWellKnownType(importName string) (string, error) {
	f, err := WellKnownTypesFS.Open(importName)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, f)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
