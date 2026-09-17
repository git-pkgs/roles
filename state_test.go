package roles_test

import (
	"errors"
	"testing"

	"github.com/git-pkgs/roles"
)

func TestStateVendorContext(t *testing.T) {
	classifier, err := roles.New([]roles.VendorRoot{
		{Path: cratesVendorRoot, Ecosystem: cargoEcosystem, EvidencePath: cargoConfig},
		{Path: sharedVendorRoot, Ecosystem: npmEcosystem, EvidencePath: packageJSON},
	})
	if err != nil {
		t.Fatal(err)
	}
	state := classifier.RootState()
	for _, component := range [][]byte{[]byte("deps"), []byte("crates"), []byte("src")} {
		state, err = state.Enter(component)
		if err != nil {
			t.Fatal(err)
		}
	}
	got, err := state.Match([]byte("lib.rs"))
	want, matchErr := classifier.Match("deps/crates/src/lib.rs")
	if err != nil || matchErr != nil || got != want || !got.Has(roles.Vendor) || !got.Has(roles.Source) {
		t.Fatalf("incremental Match = %v, want %v, errors %v, %v", got.List(), want.List(), err, matchErr)
	}
}

func TestStateKeys(t *testing.T) {
	if (roles.State{}).Key() != roles.RootState().Key() {
		t.Fatal("zero state differs from default root")
	}
	root := roles.RootState()
	github, err := root.Enter([]byte(".github"))
	if err != nil {
		t.Fatal(err)
	}
	other, err := root.Enter([]byte("other"))
	if err != nil {
		t.Fatal(err)
	}
	if github.Roles() != other.Roles() || github.Key() == other.Key() {
		t.Fatal("directory-path progress missing from key")
	}
	workflows, err := github.Enter([]byte("workflows"))
	if err != nil || !workflows.Roles().Has(roles.CI) {
		t.Fatalf("workflows state = %v, %v", workflows.Roles().List(), err)
	}
	circle, err := root.Enter([]byte(".circleci"))
	if err != nil || workflows.Key() != circle.Key() {
		t.Fatalf("equivalent CI contexts differ: %v", err)
	}

	first, err := roles.New([]roles.VendorRoot{{Path: "deps/b"}, {Path: "deps/a", Ecosystem: cargoEcosystem}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := roles.New([]roles.VendorRoot{{Path: "deps/a", Ecosystem: npmEcosystem}, {Path: "deps/b"}})
	if err != nil {
		t.Fatal(err)
	}
	if first.RootState().Key() != second.RootState().Key() {
		t.Fatal("equivalent vendor path configurations have different keys")
	}
	different, err := roles.New([]roles.VendorRoot{{Path: "third_party"}})
	if err != nil {
		t.Fatal(err)
	}
	if first.RootState().Key() == different.RootState().Key() {
		t.Fatal("different vendor path configurations have the same key")
	}
	deps, err := first.RootState().Enter([]byte("deps"))
	if err != nil || deps.Roles() != 0 || deps.Key() == first.RootState().Key() {
		t.Fatalf("vendor progress missing from key: %v", err)
	}

	cache := map[roles.StateKey]string{workflows.Key(): "ci"}
	if cache[circle.Key()] != "ci" {
		t.Fatal("state key is not usable as a map key")
	}
}

func TestStateRawNamesAndValidation(t *testing.T) {
	state, err := roles.RootState().Enter([]byte("src"))
	if err != nil {
		t.Fatal(err)
	}
	name := []byte{0xff, '.', 'g', 'o'}
	got, err := state.Match(name)
	want, matchErr := roles.Match("src/\xff.go")
	if err != nil || matchErr != nil || got != want {
		t.Fatalf("raw Match = %v, want %v, errors %v, %v", got.List(), want.List(), err, matchErr)
	}
	for _, invalid := range [][]byte{nil, {}, []byte("."), []byte(".."), []byte("a/b"), []byte{'a', 0, 'b'}} {
		if _, err := state.Enter(invalid); !errors.Is(err, roles.ErrInvalidPath) {
			t.Errorf("Enter accepted %q: %v", invalid, err)
		}
		if _, err := state.Match(invalid); !errors.Is(err, roles.ErrInvalidPath) {
			t.Errorf("Match accepted %q: %v", invalid, err)
		}
	}
}

func TestStateAllocations(t *testing.T) {
	src := []byte("src")
	license := []byte("LICENSE")
	if got := testing.AllocsPerRun(100, func() {
		state, _ := roles.RootState().Enter(src)
		_, _ = state.Match(license)
	}); got != 0 {
		t.Fatalf("default state allocations = %v", got)
	}
	classifier, err := roles.New([]roles.VendorRoot{{Path: cratesVendorRoot}})
	if err != nil {
		t.Fatal(err)
	}
	deps := []byte("deps")
	crates := []byte("crates")
	if got := testing.AllocsPerRun(100, func() {
		state, _ := classifier.RootState().Enter(deps)
		_, _ = state.Enter(crates)
	}); got != 0 {
		t.Fatalf("vendor state allocations = %v", got)
	}
}

func incrementalMatch(name string) (roles.Set, error) {
	state := roles.RootState()
	directory := name[len(name)-1] == '/'
	end := len(name)
	if directory {
		end--
	}
	start := 0
	for i := 0; i < end; i++ {
		if name[i] != '/' {
			continue
		}
		var err error
		state, err = state.Enter([]byte(name[start:i]))
		if err != nil {
			return 0, err
		}
		start = i + 1
	}
	if directory {
		next, err := state.Enter([]byte(name[start:end]))
		return next.Roles(), err
	}
	return state.Match([]byte(name[start:end]))
}

func assertIncrementalMatch(t *testing.T, name string, want roles.Set) {
	t.Helper()
	got, err := incrementalMatch(name)
	if err != nil || got != want {
		t.Fatalf("incremental Match = %v, want %v, error %v", got.List(), want.List(), err)
	}
}
