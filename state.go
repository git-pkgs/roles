package roles

import (
	"crypto/sha256"
	"strings"
	"unicode/utf8"
)

const maxPathProgress = 64

// State incrementally classifies one repository path. Its zero value is the
// root state for the package-level classifier.
type State struct {
	classifier     *Classifier
	set            Set
	pathProgress   uint64
	vendorProgress uint32
}

// StateKey is a stable, comparable summary of the context that can affect
// descendant matches. It can be combined with a Git tree object ID for a
// semantic subtree cache.
type StateKey struct {
	context        stateContext
	set            Set
	pathProgress   uint64
	vendorProgress uint32
}

type incrementalPath struct {
	rule   *rule
	parts  []string
	offset uint8
}

type vendorTrieNode struct {
	children map[string]uint32
	terminal bool
}

type stateContext [sha256.Size]byte

var (
	incrementalPaths    []incrementalPath
	pathProgressBits    int
	defaultStateContext = makeStateContext(nil)
)

// RootState returns the package-level classifier's root state.
func RootState() State { return defaults.RootState() }

// RootState returns this classifier's root state.
func (c *Classifier) RootState() State {
	state := State{classifier: c}
	if len(c.vendorTrie) > 0 {
		state.vendorProgress = 1
	}
	return state
}

// Enter returns the state for a child directory component. Git tree names may
// contain arbitrary non-NUL bytes but cannot contain a slash.
func (s State) Enter(name []byte) (State, error) {
	if err := validComponent(name); err != nil {
		return State{}, err
	}
	return s.enterValid(name), nil
}

// Match returns the roles for a child file component without changing the
// parent state.
func (s State) Match(name []byte) (Set, error) {
	if err := validComponent(name); err != nil {
		return 0, err
	}
	return s.matchValid(name), nil
}

func (s State) enterValid(name []byte) State {
	next := s
	matchDirectoryBytes(&next, name)
	return next
}

func (s State) matchValid(name []byte) Set {
	state := matchState{set: s.set}
	matchFileBytes(&state, name)
	return state.set
}

// Roles returns the roles accumulated by entered directories.
func (s State) Roles() Set { return s.set }

// Key returns the semantic context needed to cache descendant tree matches.
func (s State) Key() StateKey {
	return StateKey{
		context:        s.context(),
		set:            s.set,
		pathProgress:   s.pathProgress,
		vendorProgress: s.vendorProgress,
	}
}

func (s State) context() stateContext {
	if s.classifier == nil || s.classifier.stateContext == (stateContext{}) {
		return defaultStateContext
	}
	return s.classifier.stateContext
}

func (s State) activeClassifier() *Classifier {
	if s.classifier == nil {
		return defaults
	}
	return s.classifier
}

func validComponent(name []byte) error {
	if len(name) == 0 || equalBytesString(name, ".") || equalBytesString(name, "..") {
		return ErrInvalidPath
	}
	for _, value := range name {
		if value == 0 || value == '/' {
			return ErrInvalidPath
		}
	}
	return nil
}

func matchDirectoryBytes(state *State, name []byte) {
	for _, r := range directoryRules[string(name)] {
		state.set |= r.bit
	}
	for _, r := range foldedDirectories {
		if equalFoldBytesString(name, r.Pattern) {
			state.set |= r.bit
		}
	}
	matchPathDirectory(state, name)
	matchVendorDirectory(state, name)
}

func matchPathDirectory(state *State, name []byte) {
	previous := state.pathProgress
	state.pathProgress = 0
	for _, path := range incrementalPaths {
		if equalBytesString(name, path.parts[0]) {
			if len(path.parts) == 1 {
				state.set |= path.rule.bit
			} else {
				state.pathProgress |= uint64(1) << path.offset
			}
		}
		for part := 1; part < len(path.parts); part++ {
			progress := uint64(1) << (int(path.offset) + part - 1)
			if previous&progress == 0 || !equalBytesString(name, path.parts[part]) {
				continue
			}
			if part == len(path.parts)-1 {
				state.set |= path.rule.bit
			} else {
				state.pathProgress |= progress << 1
			}
		}
	}
}

func matchVendorDirectory(state *State, name []byte) {
	if state.vendorProgress == 0 {
		return
	}
	trie := state.activeClassifier().vendorTrie
	node := trie[state.vendorProgress-1]
	next, ok := node.children[string(name)]
	if !ok {
		state.vendorProgress = 0
		return
	}
	if trie[next].terminal {
		state.set |= roleBit(Vendor)
		state.vendorProgress = 0
		return
	}
	state.vendorProgress = next + 1
}

func matchFileBytes(state *matchState, base []byte) {
	for _, r := range filenameRules[string(base)] {
		state.set |= r.bit
	}
	stem, ext := base, []byte(nil)
	for i, value := range base {
		if value == '.' {
			stem, ext = base[:i], base[i:]
			break
		}
	}
	for _, r := range foldedStems[len(stem)] {
		if equalFoldBytesString(stem, r.Pattern) && extensionMatchesBytes(ext, r.Extensions) {
			state.set |= r.bit
		}
	}
	for _, r := range foldedPrefixes {
		if prefixMatchesBytes(base, r.Pattern) {
			state.set |= r.bit
		}
	}
	for _, r := range suffixRules[suffixBucket(base[len(base)-1])] {
		if suffixMatchesBytes(base, r) {
			state.set |= r.bit
		}
	}
}

func registerIncrementalPath(r *rule) {
	parts := strings.Split(r.Pattern, "/")
	partial := len(parts) - 1
	if pathProgressBits+partial > maxPathProgress {
		panic("too many directory-path rule components")
	}
	incrementalPaths = append(incrementalPaths, incrementalPath{rule: r, parts: parts, offset: uint8(pathProgressBits)})
	pathProgressBits += partial
}

func buildVendorTrie(paths []string) []vendorTrieNode {
	if len(paths) == 0 {
		return nil
	}
	nodes := []vendorTrieNode{{children: map[string]uint32{}}}
	for _, path := range paths {
		current := uint32(0)
		for part := range strings.SplitSeq(path, "/") {
			next, ok := nodes[current].children[part]
			if !ok {
				next = uint32(len(nodes))
				nodes[current].children[part] = next
				nodes = append(nodes, vendorTrieNode{children: map[string]uint32{}})
			}
			current = next
		}
		nodes[current].terminal = true
	}
	return nodes
}

func makeStateContext(paths []string) stateContext {
	var input strings.Builder
	input.WriteString(CorpusVersion)
	for _, path := range paths {
		input.WriteByte(0)
		input.WriteString(path)
	}
	return stateContext(sha256.Sum256([]byte(input.String())))
}

func equalBytesString(value []byte, pattern string) bool {
	if len(value) != len(pattern) {
		return false
	}
	for i, current := range value {
		if current != pattern[i] {
			return false
		}
	}
	return true
}

func equalFoldBytesString(value []byte, pattern string) bool {
	if allASCII(value) {
		if len(value) != len(pattern) {
			return false
		}
		for i, current := range value {
			if foldASCII(current) != foldASCII(pattern[i]) {
				return false
			}
		}
		return true
	}
	return strings.EqualFold(string(value), pattern)
}

func allASCII(value []byte) bool {
	for _, current := range value {
		if current >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

func foldASCII(value byte) byte {
	if value >= 'A' && value <= 'Z' {
		return value + ('a' - 'A')
	}
	return value
}

func extensionMatchesBytes(ext []byte, extensions []string) bool {
	for _, candidate := range extensions {
		if equalFoldBytesString(ext, candidate) {
			return true
		}
	}
	return false
}

func prefixMatchesBytes(name []byte, prefix string) bool {
	if len(name) < len(prefix) {
		return false
	}
	if len(name) > len(prefix) {
		switch name[len(prefix)] {
		case '.', '-', '_':
		default:
			return false
		}
	}
	return equalFoldBytesString(name[:len(prefix)], prefix)
}

func suffixMatchesBytes(base []byte, r *rule) bool {
	if len(base) < len(r.Pattern) {
		return false
	}
	suffix := base[len(base)-len(r.Pattern):]
	if r.Kind == suffixFoldKind {
		return equalFoldBytesString(suffix, r.Pattern)
	}
	return equalBytesString(suffix, r.Pattern)
}
