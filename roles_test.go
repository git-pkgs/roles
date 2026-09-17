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

const (
	cargoConfig          = ".cargo/config.toml"
	secondaryCargoConfig = "config/cargo.toml"
	cargoEcosystem       = "cargo"
	npmEcosystem         = "npm"
	packageJSON          = "package.json"
	cratesVendorRoot     = "deps/crates"
	sharedVendorRoot     = "deps/shared"
	contextSource        = "context"
	contextVendorRule    = "context.vendor-root"
)

func TestCorpus(t *testing.T) {
	for _, file := range []string{"testdata/paths.json", "testdata/gitignore-paths.json", "testdata/linguist-paths.json"} {
		t.Run(file, func(t *testing.T) { testCorpusFile(t, file) })
	}
}

func testCorpusFile(t *testing.T, file string) {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Path     string
		Roles    []roles.Role
		Subtypes []string
		Sources  []string
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
			for _, source := range tc.Sources {
				if !slices.ContainsFunc(got.Evidence, func(e roles.Evidence) bool { return e.Source == source }) {
					t.Errorf("missing source %s", source)
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
		{Rule: "vendor.directory.vendor", Role: roles.Vendor, Path: "vendor", Source: "roles"},
		{Rule: "legal.directory-fold.licenses", Role: roles.Legal, Path: "vendor/LICENSES", Subtype: "license", Source: "licenses"},
		{Rule: "legal.prefix-fold.notice", Role: roles.Legal, Path: "vendor/LICENSES/NOTICE", Subtype: "notice", Source: "licenses"},
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
	roots := []roles.VendorRoot{{Path: "deps/local", Ecosystem: cargoEcosystem, EvidencePath: cargoConfig}}
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
	if got.Evidence[0].Source != contextSource || got.Evidence[0].EvidencePath != cargoConfig {
		t.Fatal(got.Evidence)
	}
	encoded, err := json.Marshal(got.Evidence[0])
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["source"] != contextSource || fields["evidence_path"] != cargoConfig {
		t.Fatal(fields)
	}
	if _, exists := fields["origin"]; exists {
		t.Fatal(fields)
	}
	other, _ := c.Classify("deps/localish/src/lib.rs")
	if other.Has(roles.Vendor) {
		t.Fatal("root boundary")
	}
	for _, roots := range [][]roles.VendorRoot{{{Path: "../bad"}}, {{Path: "ok", EvidencePath: "/escape"}}} {
		if _, err := roles.New(roots); err == nil {
			t.Fatal("accepted invalid roots")
		}
	}
}

func TestVendorContextMultipleEvidence(t *testing.T) {
	roots := []roles.VendorRoot{
		{Path: sharedVendorRoot + "/", Ecosystem: npmEcosystem, EvidencePath: packageJSON},
		{Path: sharedVendorRoot, Ecosystem: cargoEcosystem, EvidencePath: secondaryCargoConfig},
		{Path: sharedVendorRoot, Ecosystem: cargoEcosystem, EvidencePath: cargoConfig},
		{Path: sharedVendorRoot, Ecosystem: npmEcosystem, EvidencePath: packageJSON},
		{Path: sharedVendorRoot + "/nested", Ecosystem: "golang", EvidencePath: "vendor/modules.txt"},
	}
	first, err := roles.New(roots)
	if err != nil {
		t.Fatal(err)
	}
	slices.Reverse(roots)
	second, err := roles.New(roots)
	if err != nil {
		t.Fatal(err)
	}
	path := sharedVendorRoot + "/nested/src/lib.go"
	got, err := first.Classify(path)
	if err != nil {
		t.Fatal(err)
	}
	again, err := second.Classify(path)
	if err != nil || !reflect.DeepEqual(got, again) {
		t.Fatalf("input order changed evidence: %#v, %v", again, err)
	}
	var context []roles.Evidence
	for _, evidence := range got.Evidence {
		if evidence.Rule == contextVendorRule {
			context = append(context, evidence)
		}
	}
	want := []roles.Evidence{
		{Rule: contextVendorRule, Role: roles.Vendor, Path: sharedVendorRoot, Ecosystem: cargoEcosystem, Source: contextSource, EvidencePath: cargoConfig},
		{Rule: contextVendorRule, Role: roles.Vendor, Path: sharedVendorRoot, Ecosystem: cargoEcosystem, Source: contextSource, EvidencePath: secondaryCargoConfig},
		{Rule: contextVendorRule, Role: roles.Vendor, Path: sharedVendorRoot, Ecosystem: npmEcosystem, Source: contextSource, EvidencePath: packageJSON},
		{Rule: contextVendorRule, Role: roles.Vendor, Path: sharedVendorRoot + "/nested", Ecosystem: "golang", Source: contextSource, EvidencePath: "vendor/modules.txt"},
	}
	if !reflect.DeepEqual(context, want) {
		t.Fatalf("context evidence = %#v, want %#v", context, want)
	}
	set, err := first.Match(path)
	if err != nil || !set.Has(roles.Vendor) {
		t.Fatalf("Match = %v, %v", set.List(), err)
	}
}

func TestMatchAllocations(t *testing.T) {
	if got := testing.AllocsPerRun(100, func() { _, _ = roles.Match("packages/parser/vendor/src/parser_test.go") }); got != 0 {
		t.Fatalf("allocations = %v", got)
	}
}

func FuzzClassify(f *testing.F) {
	for _, name := range []string{"vendor/LICENSES/NOTICE", "src/a_test.go", "../x", "test/\xff", "a\xff.CSS.MAP", "a.Designer.cs\xff"} {
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

func TestGeneratedSuffixBytes(t *testing.T) {
	for name, generated := range map[string]bool{
		"a\xff.CSS.MAP":     true,
		"a.Designer.cs\xff": false,
		"a.PYC":             false,
		"a.MIN.JS":          false,
		"a.Designer.cſ":     false,
	} {
		result, err := roles.Classify(name)
		if err != nil {
			t.Fatal(err)
		}
		if got := slices.Contains(result.Roles, roles.Generated); got != generated {
			t.Errorf("Classify(%q) generated = %v, want %v", name, got, generated)
		}
	}
}
