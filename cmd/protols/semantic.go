package main

import (
	"sort"

	"github.com/bufbuild/protocompile/ast"
	"github.com/bufbuild/protocompile/parser"
	"golang.org/x/tools/gopls/pkg/lsp/protocol"
)

type tokenType uint32

const (
	semanticTypeNamespace tokenType = iota
	semanticTypeType
	semanticTypeClass
	semanticTypeEnum
	semanticTypeInterface
	semanticTypeStruct
	semanticTypeTypeParameter
	semanticTypeParameter
	semanticTypeVariable
	semanticTypeProperty
	semanticTypeEnumMember
	semanticTypeEvent
	semanticTypeFunction
	semanticTypeMethod
	semanticTypeMacro
	semanticTypeKeyword
	semanticTypeModifier
	semanticTypeComment
	semanticTypeString
	semanticTypeNumber
	semanticTypeRegexp
	semanticTypeOperator
)

type tokenModifier uint32

const (
	semanticModifierDeclaration tokenModifier = 1 << iota
	semanticModifierDefinition
	semanticModifierReadonly
	semanticModifierStatic
	semanticModifierDeprecated
	semanticModifierAbstract
	semanticModifierAsync
	semanticModifierModification
	semanticModifierDocumentation
	semanticModifierDefaultLibrary
)

type semItem struct {
	line, start uint32
	len         uint32
	typ         tokenType
	mods        tokenModifier
}

type encoded struct {
	// the generated data
	items []semItem

	res        parser.Result
	mapper     *protocol.Mapper
	rng        *protocol.Range
	start, end ast.Token
}

func semanticTokensFull(cache *Cache, doc protocol.TextDocumentIdentifier) (*protocol.SemanticTokens, error) {
	ret, err := computeSemanticTokens(cache, doc, nil)
	return ret, err
}

func semanticTokensRange(cache *Cache, doc protocol.TextDocumentIdentifier, rng protocol.Range) (*protocol.SemanticTokens, error) {
	ret, err := computeSemanticTokens(cache, doc, &rng)
	return ret, err
}

func computeSemanticTokens(cache *Cache, td protocol.TextDocumentIdentifier, rng *protocol.Range) (*protocol.SemanticTokens, error) {
	file, err := cache.FindParseResultByURI(td.URI.SpanURI())
	if err != nil {
		return nil, err
	}

	ans := protocol.SemanticTokens{
		Data: []uint32{},
	}

	mapper, err := cache.getMapper(td.URI.SpanURI())
	if err != nil {
		return nil, err
	}
	a := file.AST()
	var startToken, endToken ast.Token
	if rng == nil {
		startToken = a.Start()
		endToken = a.End()
	} else {
		startOff, endOff, _ := mapper.RangeOffsets(*rng)
		startToken = a.TokenAtOffset(startOff)
		endToken = a.TokenAtOffset(endOff)
	}

	e := &encoded{
		rng:    rng,
		res:    file,
		mapper: mapper,
		start:  startToken,
		end:    endToken,
	}
	allNodes := a.Children()
	for _, node := range allNodes {
		// only look at the decls that overlap the range
		start, end := node.Start(), node.End()
		if end <= e.start || start >= e.end {
			continue
		}
		e.inspect(node)
	}

	ans.Data = e.Data()
	return &ans, nil
}

func (s *encoded) mktokens(node ast.Node, tt tokenType, mods tokenModifier) {
	info := s.res.AST().NodeInfo(node)
	if !info.IsValid() {
		return
	}

	if node.Start() >= s.end || node.End() <= s.start {
		return
	}

	if info.End().Line != info.Start().Line {
		return
	}

	length := (info.End().Col - 1) - (info.Start().Col - 1)

	nodeTk := semItem{
		line:  uint32(info.Start().Line - 1),
		start: uint32(info.Start().Col - 1),
		len:   uint32(length),
		typ:   tt,
		mods:  mods,
	}
	s.items = append(s.items, nodeTk)

	leadingComments := info.LeadingComments()
	for i := 0; i < leadingComments.Len(); i++ {
		comment := leadingComments.Index(i)
		commentTk := semItem{
			line:  uint32(comment.Start().Line - 1),
			start: uint32(comment.Start().Col - 1),
			len:   uint32((comment.End().Col) - (comment.Start().Col - 1)),
			typ:   semanticTypeComment,
		}
		s.items = append(s.items, commentTk)
	}

	trailingComments := info.TrailingComments()
	for i := 0; i < trailingComments.Len(); i++ {
		comment := trailingComments.Index(i)
		commentTk := semItem{
			line:  uint32(comment.Start().Line - 1),
			start: uint32(comment.Start().Col - 1),
			len:   uint32((comment.End().Col) - (comment.Start().Col - 1)),
			typ:   semanticTypeComment,
		}
		s.items = append(s.items, commentTk)
	}
}

func (s *encoded) inspect(node ast.Node) {
	ast.Walk(node, &ast.SimpleVisitor{
		DoVisitStringLiteralNode: func(node *ast.StringLiteralNode) error {
			s.mktokens(node, semanticTypeString, 0)
			return nil
		},
		DoVisitUintLiteralNode: func(node *ast.UintLiteralNode) error {
			s.mktokens(node, semanticTypeNumber, 0)
			return nil
		},
		DoVisitFloatLiteralNode: func(node *ast.FloatLiteralNode) error {
			s.mktokens(node, semanticTypeNumber, 0)
			return nil
		},
		DoVisitSpecialFloatLiteralNode: func(node *ast.SpecialFloatLiteralNode) error {
			s.mktokens(node, semanticTypeNumber, 0)
			return nil
		},
		DoVisitKeywordNode: func(node *ast.KeywordNode) error {
			s.mktokens(node, semanticTypeKeyword, 0)
			return nil
		},
		DoVisitRuneNode: func(node *ast.RuneNode) error {
			switch node.Rune {
			case '{', '}', ';', '.':
			default:
				s.mktokens(node, semanticTypeOperator, 0)
			}
			return nil
		},
		DoVisitMessageNode: func(node *ast.MessageNode) error {
			s.mktokens(node.Name, semanticTypeClass, 0)
			return nil
		},
		DoVisitFieldNode: func(node *ast.FieldNode) error {
			s.mktokens(node.Name, semanticTypeProperty, 0)
			s.mktokens(node.FldType, semanticTypeType, 0)
			return nil
		},
		DoVisitFieldReferenceNode: func(node *ast.FieldReferenceNode) error {
			s.mktokens(node.Name, semanticTypeProperty, 0)
			return nil
		},
		DoVisitMapFieldNode: func(node *ast.MapFieldNode) error {
			s.mktokens(node.Name, semanticTypeProperty, 0)
			s.mktokens(node.MapType.KeyType, semanticTypeType, 0)
			s.mktokens(node.MapType.ValueType, semanticTypeType, 0)
			return nil
		},
		DoVisitRPCTypeNode: func(node *ast.RPCTypeNode) error {
			s.mktokens(node.MessageType, semanticTypeType, 0)
			return nil
		},
		DoVisitRPCNode: func(node *ast.RPCNode) error {
			s.mktokens(node.Name, semanticTypeFunction, 0)
			return nil
		},
		DoVisitServiceNode: func(sn *ast.ServiceNode) error {
			s.mktokens(sn.Name, semanticTypeInterface, 0)
			return nil
		},
		DoVisitPackageNode: func(node *ast.PackageNode) error {
			s.mktokens(node.Name, semanticTypeNamespace, 0)
			return nil
		},
		DoVisitEnumNode: func(node *ast.EnumNode) error {
			s.mktokens(node.Name, semanticTypeClass, 0)
			return nil
		},
		DoVisitEnumValueNode: func(node *ast.EnumValueNode) error {
			s.mktokens(node.Name, semanticTypeEnumMember, 0)
			return nil
		},
	})
}

func (e *encoded) Data() []uint32 {
	// binary operators, at least, will be out of order
	sort.Slice(e.items, func(i, j int) bool {
		if e.items[i].line != e.items[j].line {
			return e.items[i].line < e.items[j].line
		}
		return e.items[i].start < e.items[j].start
	})
	// each semantic token needs five values
	// (see Integer Encoding for Tokens in the LSP spec)
	x := make([]uint32, 5*len(e.items))
	var j int
	var last semItem
	for i := 0; i < len(e.items); i++ {
		item := e.items[i]
		if j == 0 {
			x[0] = e.items[0].line
		} else {
			x[j] = item.line - last.line
		}
		x[j+1] = item.start
		if j > 0 && x[j] == 0 {
			x[j+1] = item.start - last.start
		}
		x[j+2] = item.len
		x[j+3] = uint32(item.typ)
		x[j+4] = uint32(item.mods)
		j += 5
		last = item
	}
	return x[:j]
}
