package roles_test

import (
	"testing"

	"github.com/git-pkgs/roles"
)

func TestLegalNames(t *testing.T) {
	for _, tc := range []struct {
		name            string
		license, notice bool
	}{
		{"LICENSE", true, false}, {"LICENCE-MIT", true, false}, {"Notice.txt", false, true},
		{"COPYRIGHT", true, false}, {"unlicense", true, false}, {"MIT-LICENSE", true, false},
		{"licensed.go", false, false}, {"noticesome", false, false}, {"src/LICENSE", false, false},
	} {
		license, notice := roles.LegalFileName(tc.name)
		if license != tc.license || notice != tc.notice {
			t.Errorf("%q: %t %t", tc.name, license, notice)
		}
		byteLicense, byteNotice := roles.LegalFileNameBytes([]byte(tc.name))
		if byteLicense != license || byteNotice != notice {
			t.Errorf("byte predicate differs for %q: %t %t", tc.name, byteLicense, byteNotice)
		}
	}
	for _, tc := range []struct {
		name string
		want bool
	}{
		{"LICENSES", true}, {"licences", true}, {"NOTICE", false}, {"licensed", false}, {"src/LICENSES", false},
	} {
		if got := roles.IsLegalDirectory(tc.name); got != tc.want {
			t.Errorf("%q: %t", tc.name, got)
		}
		if got := roles.IsLegalDirectoryBytes([]byte(tc.name)); got != tc.want {
			t.Errorf("byte predicate %q: %t", tc.name, got)
		}
	}
	if roles.IsLegalDirectoryBytes([]byte{0xff}) {
		t.Fatal("invalid UTF-8 directory matched")
	}
	if license, notice := roles.LegalFileNameBytes([]byte{0xff}); license || notice {
		t.Fatal("invalid UTF-8 filename matched")
	}
	legalDirectory := []byte("LICENSES")
	legalFile := []byte("LICENSE-MIT")
	if got := testing.AllocsPerRun(100, func() {
		roles.IsLegalDirectoryBytes(legalDirectory)
		roles.LegalFileNameBytes(legalFile)
	}); got != 0 {
		t.Fatalf("byte predicate allocations = %v", got)
	}
}

func BenchmarkLegalFileName(b *testing.B) {
	name := []byte("LICENSE-MIT")
	b.ReportAllocs()
	for b.Loop() {
		roles.LegalFileName(string(name))
	}
}

func BenchmarkLegalFileNameBytes(b *testing.B) {
	name := []byte("LICENSE-MIT")
	b.ReportAllocs()
	for b.Loop() {
		roles.LegalFileNameBytes(name)
	}
}

func BenchmarkSetHas(b *testing.B) {
	labels, err := roles.Match("vendor/src/file_test.go")
	if err != nil {
		b.Fatal(err)
	}
	for _, role := range []roles.Role{roles.Source, roles.Vendor, roles.Configuration} {
		b.Run(string(role), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				labels.Has(role)
			}
		})
	}
}
