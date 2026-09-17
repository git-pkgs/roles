package roles

import (
	"errors"
	"io/fs"
	"strings"
)

// WalkOptions bounds traversal. Zero values select the documented defaults.
type WalkOptions struct {
	MaxEntries int
	MaxDepth   int
}

const (
	DefaultMaxEntries = 1_000_000
	DefaultMaxDepth   = 256
)

// ErrLimit indicates that a walk stopped before visiting the complete tree.
var ErrLimit = errors.New("repository traversal limit reached")

// Walk classifies regular files and directories in lexical order. It skips
// symlinks and special files, propagates errors, and never reads file contents.
// The filesystem must be rooted by the caller. fs.SkipDir and fs.SkipAll from
// visit have the same meaning as in fs.WalkDir. The root itself is not emitted.
func Walk(tree fs.FS, options WalkOptions, visit func(string, Result) error) error {
	return defaults.Walk(tree, options, visit)
}

// WalkMatch visits the same entries as Walk without collecting evidence.
func WalkMatch(tree fs.FS, options WalkOptions, visit func(string, Set) error) error {
	return defaults.WalkMatch(tree, options, visit)
}

type ancestor struct {
	path        string
	set         Set
	evidenceEnd int
	incremental State
}

// Walk reuses inherited matches and includes this classifier's vendor roots.
func (c *Classifier) Walk(tree fs.FS, options WalkOptions, visit func(string, Result) error) error {
	if visit == nil {
		return errors.New("visitor is required")
	}
	return c.walk(tree, options, true, func(name string, state matchState) error {
		result := state.result()
		if len(state.evidence) > 0 {
			result.Evidence = append([]Evidence(nil), state.evidence...)
		}
		return visit(name, result)
	})
}

// WalkMatch includes this classifier's vendor roots without collecting evidence.
func (c *Classifier) WalkMatch(tree fs.FS, options WalkOptions, visit func(string, Set) error) error {
	if visit == nil {
		return errors.New("visitor is required")
	}
	return c.walk(tree, options, false, func(name string, state matchState) error {
		return visit(name, state.set)
	})
}

func (c *Classifier) walk(tree fs.FS, options WalkOptions, explain bool, visit func(string, matchState) error) error {
	if tree == nil {
		return errors.New("filesystem is required")
	}
	if options.MaxEntries < 0 || options.MaxDepth < 0 {
		return errors.New("walk limits must be non-negative")
	}
	if options.MaxEntries == 0 {
		options.MaxEntries = DefaultMaxEntries
	}
	if options.MaxDepth == 0 {
		options.MaxDepth = DefaultMaxDepth
	}
	parents := []ancestor{{path: ".", incremental: c.RootState()}}
	entries := 0
	var evidence []Evidence
	return fs.WalkDir(tree, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if name == "." {
			return nil
		}
		entries++
		if entries > options.MaxEntries {
			return ErrLimit
		}
		if err := validPath(name); err != nil {
			return err
		}
		split := strings.LastIndexByte(name, '/')
		parent := "."
		if split >= 0 {
			parent = name[:split]
		}
		for len(parents) > 1 && parents[len(parents)-1].path != parent {
			parents = parents[:len(parents)-1]
		}
		if len(parents) > options.MaxDepth {
			return ErrLimit
		}
		if entry.Type()&fs.ModeSymlink != 0 || (!entry.IsDir() && !entry.Type().IsRegular()) {
			return nil
		}
		parentState := parents[len(parents)-1]
		directory := entry.IsDir()
		state, incremental := c.matchWalkEntry(directory, parentState, name[split+1:], name, evidence, explain)
		if directory {
			parents = append(parents, ancestor{path: name, set: state.set, evidenceEnd: len(state.evidence), incremental: incremental})
			name += "/"
		}
		evidence = state.evidence
		return visit(name, state)
	})
}

func (c *Classifier) matchWalkEntry(directory bool, parent ancestor, base, full string, evidence []Evidence, explain bool) (matchState, State) {
	state := matchState{set: parent.set, evidence: evidence[:parent.evidenceEnd]}
	incremental := parent.incremental
	switch {
	case directory && explain:
		c.directory(&state, base, full, true)
	case directory:
		incremental = incremental.enterValid([]byte(base))
		state.set = incremental.Roles()
	case explain:
		matchFile(&state, base, full, true)
	default:
		state.set = incremental.matchValid([]byte(base))
	}
	return state, incremental
}
