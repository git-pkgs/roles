package roles

import (
	"bytes"
	"go/scanner"
	"go/token"
	"strings"
)

const (
	MaxHeaderBytes = 8192
	MaxHeaderLines = 40
)

// BlobResult describes the bounded Go header inspection, where applicable.
// Absence of Generated is not proof that a file was handwritten.
type BlobResult struct {
	Result
	HeaderChecked bool `json:"header_checked"`
	BytesExamined int  `json:"bytes_examined"`
	HeaderLimited bool `json:"header_limited"`
}

// ClassifyBlob adds generated Go header evidence to path classification.
// Contents must be the complete file. At most MaxHeaderBytes and MaxHeaderLines
// are inspected. Other languages receive path-only classification.
func ClassifyBlob(name string, contents []byte) (BlobResult, error) {
	return defaults.ClassifyBlob(name, contents)
}

// ClassifyBlob preserves this classifier's vendor-root context.
func (c *Classifier) ClassifyBlob(name string, contents []byte) (BlobResult, error) {
	result, err := c.Classify(name)
	if err != nil {
		return BlobResult{}, err
	}
	blob := BlobResult{Result: result}
	if !strings.HasSuffix(name, ".go") {
		return blob, nil
	}
	blob.HeaderChecked = true
	header := contents[:min(len(contents), MaxHeaderBytes)]
	lines := 0
	for i, char := range header {
		if char == '\n' {
			lines++
		}
		if lines == MaxHeaderLines {
			header = header[:i+1]
			break
		}
	}
	blob.HeaderLimited = len(header) < len(contents)
	blob.BytesExamined = len(header)
	file := token.NewFileSet().AddFile(name, -1, len(header))
	var lexer scanner.Scanner
	lexer.Init(file, header, nil, scanner.ScanComments)
	for {
		pos, kind, literal := lexer.Scan()
		if kind != token.COMMENT {
			return blob, nil
		}
		if !strings.HasPrefix(literal, "// Code generated ") || !strings.HasSuffix(literal, " DO NOT EDIT.") {
			continue
		}
		if bytes.IndexByte(header[file.Offset(pos):], '\n') < 0 {
			continue
		}
		if !blob.Has(Generated) {
			blob.Roles = (setOf(blob.Roles) | roleBit(Generated)).List()
		}
		blob.Evidence = append(blob.Evidence, Evidence{Rule: "generated.go-header", Role: Generated, Path: name, Subtype: "header", Ecosystem: "go", Origin: "roles"})
		return blob, nil
	}
}

func setOf(roles []Role) Set {
	var set Set
	for _, role := range roles {
		set |= roleBit(role)
	}
	return set
}
