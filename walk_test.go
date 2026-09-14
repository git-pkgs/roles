package roles_test

import (
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/git-pkgs/roles"
)

func TestWalk(t *testing.T) {
	tree := fstest.MapFS{
		"packages/api/.buildkite/pipeline.yml":       {},
		"packages/api/Form.DESIGNER.CS":              {},
		"packages/api/.pytest_cache/v/cache/nodeids": {},
		"packages/api/Support.markdown":              {},
		"vendor/src/a_test.go":                       {Data: []byte("package a")},
		"src/a.go":                                   {Data: []byte("package a")},
		"LICENSES/NOTICE":                            {},
		"modules/a/testdata/package-lock.json":       {},
		"modules/b/testdata/package-lock.json":       {},
		"deps/crates/src/lib.rs":                     {},
		"link":                                       {Mode: fs.ModeSymlink, Data: []byte("../outside")},
	}
	c, err := roles.New([]roles.VendorRoot{{Path: "deps/crates", EvidencePath: cargoConfig}})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	err = c.Walk(tree, roles.WalkOptions{}, func(name string, got roles.Result) error {
		if name == "link" {
			t.Fatal("visited symlink")
		}
		want, err := c.Classify(name)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: got %#v want %#v", name, got, want)
		}
		count++
		if len(got.Evidence) > 0 {
			got.Evidence[0].Path = "mutated"
		}
		return nil
	})
	if err != nil || count != 28 {
		t.Fatalf("count %d, error %v", count, err)
	}
}

func TestWalkLimitsAndErrors(t *testing.T) {
	tree := fstest.MapFS{"a/b/c.go": {}, "other.go": {}}
	for _, opts := range []roles.WalkOptions{{MaxEntries: 1}, {MaxDepth: 1}} {
		if err := roles.Walk(tree, opts, func(string, roles.Result) error { return nil }); !errors.Is(err, roles.ErrLimit) {
			t.Fatalf("limit: %v", err)
		}
	}
	failure := errors.New("write failed")
	if err := roles.Walk(tree, roles.WalkOptions{}, func(string, roles.Result) error { return failure }); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if err := roles.Walk(failingFS{}, roles.WalkOptions{}, func(string, roles.Result) error { return nil }); !errors.Is(err, fs.ErrPermission) {
		t.Fatal(err)
	}
	if err := roles.Walk(tree, roles.WalkOptions{MaxEntries: -1}, func(string, roles.Result) error { return nil }); err == nil {
		t.Fatal("accepted negative limit")
	}
}

type failingFS struct{}

func (failingFS) Open(string) (fs.File, error) { return nil, fs.ErrPermission }

func TestWalkPruning(t *testing.T) {
	tree := fstest.MapFS{"vendor/x.go": {}, "src/main.go": {}}
	var names []string
	err := roles.Walk(tree, roles.WalkOptions{}, func(name string, _ roles.Result) error {
		names = append(names, name)
		if name == "vendor/" {
			return fs.SkipDir
		}
		return nil
	})
	if err != nil || strings.Contains(strings.Join(names, ","), "vendor/x.go") {
		t.Fatal(names, err)
	}
	count := 0
	err = roles.Walk(tree, roles.WalkOptions{}, func(string, roles.Result) error { count++; return fs.SkipAll })
	if err != nil || count != 1 {
		t.Fatal(count, err)
	}
}

func FuzzWalk(f *testing.F) {
	f.Add("vendor/src/a.go", "tests/fixtures/b.json")
	f.Fuzz(func(t *testing.T, first, second string) {
		if len(first)+len(second) > 1024 {
			return
		}
		tree := fstest.MapFS{}
		for _, name := range []string{first, second} {
			if !fs.ValidPath(name) || name == "." || strings.Count(name, "/") > 16 {
				return
			}
			tree[name] = &fstest.MapFile{}
		}
		if strings.HasPrefix(first, second+"/") || strings.HasPrefix(second, first+"/") {
			return
		}
		err := roles.Walk(tree, roles.WalkOptions{}, func(name string, got roles.Result) error {
			want, err := roles.Classify(name)
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("path/tree disagreement: %q", name)
			}
			return nil
		})
		if err != nil && !errors.Is(err, roles.ErrInvalidPath) {
			t.Fatal(err)
		}
	})
}

func TestWalkRetainedResults(t *testing.T) {
	tree := fstest.MapFS{
		"vendor/src/test/LICENSE": {},
		"vendor/src/test/NOTICE":  {},
		"vendor/src/other.go":     {},
		"other/LICENSES/NOTICE":   {},
		"plain":                   {},
	}
	retained := map[string]roles.Result{}
	err := roles.Walk(tree, roles.WalkOptions{}, func(name string, result roles.Result) error {
		retained[name] = result
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for name, result := range retained {
		expected, err := roles.Classify(name)
		if err != nil || !reflect.DeepEqual(result, expected) {
			t.Fatalf("retained result changed: %q, %v", name, err)
		}
	}
}

func TestWalkDepthAfterPruning(t *testing.T) {
	tree := fstest.MapFS{
		"a/skip/deep/file": {},
		"b/file":           {},
	}
	var visited []string
	err := roles.Walk(tree, roles.WalkOptions{MaxDepth: 2}, func(name string, _ roles.Result) error {
		visited = append(visited, name)
		if name == "a/skip/" {
			return fs.SkipDir
		}
		return nil
	})
	if err != nil || !reflect.DeepEqual(visited, []string{"a/", "a/skip/", "b/", "b/file"}) {
		t.Fatal(visited, err)
	}
}
