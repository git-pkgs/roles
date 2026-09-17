package roles_test

import (
	"strings"
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
		{"LICENSES", true}, {legalLicences, true}, {"NOTICE", false}, {"licensed", false}, {"src/LICENSES", false},
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

func TestLegalNameCompatibility(t *testing.T) {
	assertLegalDirectories(t)
	assertLegalFilePrefixes(t, []string{licensesValue, legalLicense, legalLicences, "licence", "copying", "mit-license", "copyright", "unlicense"}, true)
	assertLegalFilePrefixes(t, []string{legalNotice, "notices"}, false)
}

func assertLegalDirectories(t *testing.T) {
	t.Helper()
	for _, directory := range []string{legalLicense, licensesValue, "licence", legalLicences} {
		for _, name := range []string{directory, strings.ToUpper(directory)} {
			if !roles.IsLegalDirectory(name) || !roles.IsLegalDirectoryBytes([]byte(name)) {
				t.Errorf("legal directory %q did not match", name)
			}
		}
	}
	for _, name := range []string{"licensed", "licenses-old", legalNotice, "copying"} {
		if roles.IsLegalDirectory(name) || roles.IsLegalDirectoryBytes([]byte(name)) {
			t.Errorf("non-legal directory %q matched", name)
		}
	}
}

func assertLegalFilePrefixes(t *testing.T, prefixes []string, licenseExpected bool) {
	t.Helper()
	noticeExpected := !licenseExpected
	for _, prefix := range prefixes {
		for _, suffix := range []string{"", ".txt", "-old", "_third-party"} {
			name := strings.ToUpper(prefix) + suffix
			license, notice := roles.LegalFileName(name)
			byteLicense, byteNotice := roles.LegalFileNameBytes([]byte(name))
			if license != licenseExpected || notice != noticeExpected || byteLicense != license || byteNotice != notice {
				t.Errorf("legal name %q = %t, %t; bytes = %t, %t", name, license, notice, byteLicense, byteNotice)
			}
		}
		name := prefix + "x"
		license, notice := roles.LegalFileName(name)
		if license || notice {
			t.Errorf("unbounded legal prefix %q matched", name)
		}
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
