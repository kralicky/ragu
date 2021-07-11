package machinery

import (
	"os"

	"github.com/yoheimuta/go-protoparser/v4"
	"github.com/yoheimuta/go-protoparser/v4/parser"
)

func ParseProto(filename string) (*parser.Proto, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	return protoparser.Parse(f, protoparser.WithFilename(filename))
}
