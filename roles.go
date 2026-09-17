// Package roles adds repository context to file occurrences through path
// classification. Roles are additive; classification never excludes a file.
// Paths are repository-relative, use slash separators, and end in a slash for
// directories. Optional evidence explains each matching convention.
package roles

import (
	"errors"
	"math/bits"
	"strings"
)

// Role describes a convention at a file or directory occurrence.
type Role string

// Keep explicit Role types so the constant-to-bit mapping test includes each role.
const (
	Source        Role = "source"
	Test          Role = "test"
	Fixture       Role = "fixture"
	Example       Role = "example"
	Benchmark     Role = "benchmark"
	Fuzz          Role = "fuzz"
	Vendor        Role = "vendor"
	Generated     Role = "generated"
	Documentation Role = "documentation"
	Legal         Role = "legal"
	Build         Role = "build"
	CI            Role = "ci"
	Packaging     Role = "packaging"
	Tooling       Role = "tooling"
	Configuration Role = "configuration"
)

var roleOrder = [...]Role{Source, Test, Fixture, Example, Benchmark, Fuzz, Vendor, Generated, Documentation, Legal, Build, CI, Packaging, Tooling, Configuration}

// Set is a compact collection of roles in corpus-defined order.
type Set uint32

// Has reports whether the set contains role. Unknown role names return false.
func (s Set) Has(role Role) bool { return s&roleBit(role) != 0 }

// List returns the roles in deterministic order.
func (s Set) List() []Role {
	if s == 0 {
		return nil
	}
	result := make([]Role, 0, bits.OnesCount32(uint32(s)))
	for i, role := range roleOrder {
		if s&(1<<i) != 0 {
			result = append(result, role)
		}
	}
	return result
}

var roleBits = func() map[Role]Set {
	result := make(map[Role]Set, len(roleOrder))
	for i, role := range roleOrder {
		result[role] = 1 << i
	}
	return result
}()

func roleBit(role Role) Set { return roleBits[role] }

// Evidence identifies a matched corpus rule or caller-supplied vendor root.
// Source names the rule provider. EvidencePath names the repository input that
// supplied contextual evidence, when one exists.
type Evidence struct {
	Rule         string `json:"rule"`
	Role         Role   `json:"role"`
	Path         string `json:"path"`
	Subtype      string `json:"subtype,omitempty"`
	Ecosystem    string `json:"ecosystem,omitempty"`
	Source       string `json:"source"`
	EvidencePath string `json:"evidence_path,omitempty"`
}

// Result contains labels and the independent matches that produced them.
type Result struct {
	Roles    []Role     `json:"roles"`
	Evidence []Evidence `json:"evidence"`
}

// Has reports whether the result contains role.
func (r Result) Has(role Role) bool {
	for _, candidate := range r.Roles {
		if role == candidate {
			return true
		}
	}
	return false
}

// ErrInvalidPath indicates an absolute, unclean, empty, or NUL-containing path.
var ErrInvalidPath = errors.New("expected a clean repository-relative path")

// Classify returns additive roles and all matching evidence, in stable order.
// Unknown conventions produce an empty result. A trailing slash denotes a directory.
// Paths use slash separators; backslashes and non-UTF-8 bytes are literal names.
func Classify(name string) (Result, error) { return defaults.Classify(name) }

// Match returns the same labels as Classify without collecting evidence.
// It returns ErrInvalidPath for paths outside the repository-relative contract.
func Match(name string) (Set, error) { return defaults.Match(name) }

// VendorRoot adds a vendor directory discovered by a caller.
type VendorRoot struct {
	Path         string
	Ecosystem    string
	EvidencePath string
}

// Classifier combines the embedded corpus with immutable vendor-root context.
// A classifier can be shared by concurrent callers.
type Classifier struct{ roots map[string]VendorRoot }

var defaults = &Classifier{}

// New validates and copies caller-supplied vendor roots. Duplicate roots fail.
func New(roots []VendorRoot) (*Classifier, error) {
	c := &Classifier{roots: make(map[string]VendorRoot, len(roots))}
	for _, root := range roots {
		if err := validPath(root.Path); err != nil {
			return nil, err
		}
		root.Path = strings.TrimSuffix(root.Path, "/")
		if root.EvidencePath != "" {
			if err := validPath(root.EvidencePath); err != nil {
				return nil, err
			}
		}
		if _, exists := c.roots[root.Path]; exists {
			return nil, errors.New("duplicate vendor root")
		}
		c.roots[root.Path] = root
	}
	return c, nil
}

func validPath(name string) error {
	if name == "" || name[0] == '/' || strings.IndexByte(name, 0) >= 0 {
		return ErrInvalidPath
	}
	clean := strings.TrimSuffix(name, "/")
	for part := range strings.SplitSeq(clean, "/") {
		if part == "" || part == "." || part == ".." {
			return ErrInvalidPath
		}
	}
	return nil
}

// Match returns labels without collecting evidence.
func (c *Classifier) Match(name string) (Set, error) {
	if err := validPath(name); err != nil {
		return 0, err
	}
	state := c.scan(name, false)
	return state.set, nil
}

// Classify returns labels and evidence for a file or trailing-slash directory.
func (c *Classifier) Classify(name string) (Result, error) {
	if err := validPath(name); err != nil {
		return Result{}, err
	}
	return c.scan(name, true).result(), nil
}

type matchState struct {
	set      Set
	evidence []Evidence
}

func (s matchState) result() Result { return Result{Roles: s.set.List(), Evidence: s.evidence} }

func (s *matchState) add(r *rule, matchedPath string, explain bool) {
	s.set |= r.bit
	if explain {
		s.evidence = append(s.evidence, Evidence{Rule: r.ID, Role: r.Role, Path: matchedPath, Subtype: r.Subtype, Ecosystem: r.Ecosystem, Source: r.Source})
	}
}

func (c *Classifier) scan(name string, explain bool) matchState {
	var state matchState
	start := 0
	for i := 0; i < len(name); i++ {
		if name[i] != '/' {
			continue
		}
		c.directory(&state, name[start:i], name[:i], explain)
		start = i + 1
	}
	if start < len(name) {
		matchFile(&state, name[start:], name, explain)
	}
	return state
}

func (c *Classifier) directory(state *matchState, base, full string, explain bool) {
	for _, r := range directoryRules[base] {
		state.add(r, full, explain)
	}
	for _, r := range foldedDirectories {
		if strings.EqualFold(base, r.Pattern) {
			state.add(r, full, explain)
		}
	}
	for _, r := range pathDirectories[base] {
		if full == r.Pattern || strings.HasSuffix(full, r.directorySuffix) {
			state.add(r, full, explain)
		}
	}
	if root, ok := c.roots[full]; ok {
		state.set |= roleBit(Vendor)
		if explain {
			state.evidence = append(state.evidence, Evidence{Rule: "context.vendor-root", Role: Vendor, Path: full, Ecosystem: root.Ecosystem, Source: "context", EvidencePath: root.EvidencePath})
		}
	}
}

func matchFile(state *matchState, base, full string, explain bool) {
	for _, r := range filenameRules[base] {
		state.add(r, full, explain)
	}
	stem, ext := base, ""
	if dot := strings.IndexByte(base, '.'); dot >= 0 {
		stem, ext = base[:dot], base[dot:]
	}
	for _, r := range foldedStems[len(stem)] {
		if strings.EqualFold(stem, r.Pattern) && extensionMatches(ext, r.Extensions) {
			state.add(r, full, explain)
		}
	}
	for _, r := range foldedPrefixes {
		if prefixMatches(base, r.Pattern) {
			state.add(r, full, explain)
		}
	}
	for _, r := range suffixRules[suffixBucket(base[len(base)-1])] {
		if suffixMatches(base, r) {
			state.add(r, full, explain)
		}
	}
}

func suffixMatches(base string, r *rule) bool {
	if r.Kind == "suffix-fold" {
		return len(base) >= len(r.Pattern) && strings.EqualFold(base[len(base)-len(r.Pattern):], r.Pattern)
	}
	return strings.HasSuffix(base, r.Pattern)
}

func extensionMatches(ext string, extensions []string) bool {
	for _, candidate := range extensions {
		if strings.EqualFold(ext, candidate) {
			return true
		}
	}
	return false
}
