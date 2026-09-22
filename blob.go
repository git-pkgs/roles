package roles

import (
	"bytes"
	"errors"
	"fmt"
	"go/scanner"
	"go/token"
	"io"
	"strings"
)

const (
	MaxHeaderBytes  = 8192
	MaxHeaderLines  = 40
	ContentVersion  = "2"
	headerReadLimit = MaxHeaderBytes + 1
)

// BlobResult describes bounded content inspection, where applicable.
// Absence of Generated is not proof that a file was handwritten.
type BlobResult struct {
	Result
	HeaderChecked bool `json:"header_checked"`
	BytesExamined int  `json:"bytes_examined"`
	HeaderLimited bool `json:"header_limited"`
}

// ClassifyBlob adds generated and minified content evidence to path classification.
// Contents must be the complete file. At most MaxHeaderBytes and MaxHeaderLines
// are inspected. Unsupported file types receive path-only classification.
func ClassifyBlob(name string, contents []byte) (BlobResult, error) {
	return defaults.ClassifyBlob(name, contents)
}

// ClassifyReader adds content evidence using bounded input.
// It reads at most MaxHeaderBytes plus one byte used to detect truncation.
func ClassifyReader(name string, reader io.Reader) (BlobResult, error) {
	return defaults.ClassifyReader(name, reader)
}

// ClassifyBlob preserves this classifier's vendor-root context.
func (c *Classifier) ClassifyBlob(name string, contents []byte) (BlobResult, error) {
	blob, inspect, err := c.blobResult(name)
	if err != nil || !inspect {
		return blob, err
	}
	return inspectContent(blob, name, contents), nil
}

// ClassifyReader preserves this classifier's vendor-root context while
// bounding content reads.
func (c *Classifier) ClassifyReader(name string, reader io.Reader) (BlobResult, error) {
	blob, inspect, err := c.blobResult(name)
	if err != nil {
		return BlobResult{}, err
	}
	if reader == nil {
		return BlobResult{}, errors.New("reader is required")
	}
	if !inspect {
		return blob, nil
	}
	contents, err := io.ReadAll(io.LimitReader(reader, headerReadLimit))
	if err != nil {
		return BlobResult{}, fmt.Errorf("read content header: %w", err)
	}
	return inspectContent(blob, name, contents), nil
}

func (c *Classifier) blobResult(name string) (BlobResult, bool, error) {
	result, err := c.Classify(name)
	if err != nil {
		return BlobResult{}, false, err
	}
	blob := BlobResult{Result: result}
	return blob, contentType(name) != "", nil
}

func inspectContent(blob BlobResult, name string, contents []byte) BlobResult {
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
	if contentType(name) == "go" {
		return inspectGoHeader(blob, name, header)
	}
	return inspectSourceHeader(blob, name, header)
}

func inspectGoHeader(blob BlobResult, name string, header []byte) BlobResult {
	file := token.NewFileSet().AddFile(name, -1, len(header))
	var lexer scanner.Scanner
	lexer.Init(file, header, nil, scanner.ScanComments)
	for {
		pos, kind, literal := lexer.Scan()
		if kind != token.COMMENT {
			return blob
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
		blob.Evidence = append(blob.Evidence, Evidence{Rule: "generated.go-header", Role: Generated, Path: name, Subtype: "header", Ecosystem: "go", Source: "roles"})
		return blob
	}
}

func setOf(roles []Role) Set {
	var set Set
	for _, role := range roles {
		set |= roleBit(role)
	}
	return set
}
