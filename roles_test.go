package roles_test

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"slices"
	"testing"

	"github.com/git-pkgs/roles"
)

const cargoConfig = ".cargo/config.toml"

func TestCorpus(t *testing.T) {
	data, err := os.ReadFile("testdata/paths.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Path     string
		Roles    []roles.Role
		Subtypes []string
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Path, func(t *testing.T) {
			got, err := roles.Classify(tc.Path)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got.Roles, tc.Roles) {
				t.Fatalf("roles = %v, want %v", got.Roles, tc.Roles)
			}
			set, err := roles.Match(tc.Path)
			if err != nil || !slices.Equal(set.List(), got.Roles) {
				t.Fatalf("Match = %v, %v", set.List(), err)
			}
			for _, subtype := range tc.Subtypes {
				if !slices.ContainsFunc(got.Evidence, func(e roles.Evidence) bool { return e.Subtype == subtype }) {
					t.Errorf("missing subtype %s", subtype)
				}
			}
			again, _ := roles.Classify(tc.Path)
			if !reflect.DeepEqual(got, again) {
				t.Fatal("unstable result")
			}
		})
	}
}

func TestPathContract(t *testing.T) {
	for _, name := range []string{"", "/foo", ".", "..", "a/../b", "a/./b", "a//b", "a//", "a\x00b"} {
		if _, err := roles.Classify(name); !errors.Is(err, roles.ErrInvalidPath) {
			t.Errorf("accepted %q", name)
		}
		if _, err := roles.Match(name); !errors.Is(err, roles.ErrInvalidPath) {
			t.Errorf("Match accepted %q", name)
		}
	}
	for _, name := range []string{"src/\xff.go", "a\\b", "C:literal", "src/a\nb"} {
		if _, err := roles.Classify(name); err != nil {
			t.Errorf("rejected literal Git name %q: %v", name, err)
		}
	}
}

func TestEvidence(t *testing.T) {
	got, err := roles.Classify("vendor/LICENSES/NOTICE")
	if err != nil {
		t.Fatal(err)
	}
	want := []roles.Evidence{
		{Rule: "vendor.directory.vendor", Role: roles.Vendor, Path: "vendor", Origin: "roles"},
		{Rule: "legal.directory-fold.licenses", Role: roles.Legal, Path: "vendor/LICENSES", Subtype: "license", Origin: "licenses"},
		{Rule: "legal.prefix-fold.notice", Role: roles.Legal, Path: "vendor/LICENSES/NOTICE", Subtype: "notice", Origin: "licenses"},
	}
	if !reflect.DeepEqual(got.Evidence, want) {
		t.Fatalf("evidence = %#v", got.Evidence)
	}
	got.Evidence[0].Path = "changed"
	again, _ := roles.Classify("vendor/LICENSES/NOTICE")
	if !reflect.DeepEqual(again.Evidence, want) {
		t.Fatal("mutable shared evidence")
	}
}

func TestVendorContext(t *testing.T) {
	roots := []roles.VendorRoot{{Path: "deps/local", Ecosystem: "cargo", EvidencePath: cargoConfig}}
	c, err := roles.New(roots)
	if err != nil {
		t.Fatal(err)
	}
	roots[0].Path = "changed"
	got, err := c.Classify("deps/local/src/lib.rs")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Has(roles.Vendor) || !got.Has(roles.Source) {
		t.Fatal(got)
	}
	if got.Evidence[0].Origin != cargoConfig {
		t.Fatal(got.Evidence)
	}
	other, _ := c.Classify("deps/localish/src/lib.rs")
	if other.Has(roles.Vendor) {
		t.Fatal("root boundary")
	}
	for _, roots := range [][]roles.VendorRoot{{{Path: "../bad"}}, {{Path: "ok", EvidencePath: "/escape"}}, {{Path: "a"}, {Path: "a/"}}} {
		if _, err := roles.New(roots); err == nil {
			t.Fatal("accepted invalid roots")
		}
	}
}

func TestMatchAllocations(t *testing.T) {
	if got := testing.AllocsPerRun(100, func() { _, _ = roles.Match("packages/parser/vendor/src/parser_test.go") }); got != 0 {
		t.Fatalf("allocations = %v", got)
	}
}

func FuzzClassify(f *testing.F) {
	for _, name := range []string{"vendor/LICENSES/NOTICE", "src/a_test.go", "../x", "test/\xff"} {
		f.Add(name)
	}
	f.Fuzz(func(t *testing.T, name string) {
		result, err := roles.Classify(name)
		set, matchErr := roles.Match(name)
		if !errors.Is(err, matchErr) {
			t.Fatalf("inconsistent errors: %v, %v", err, matchErr)
		}
		if err != nil {
			return
		}
		if !slices.Equal(result.Roles, set.List()) {
			t.Fatal("inconsistent labels")
		}
		for _, e := range result.Evidence {
			if !set.Has(e.Role) || e.Rule == "" {
				t.Fatal("invalid evidence")
			}
		}
	})
}
