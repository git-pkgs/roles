package roles_test

import (
	"reflect"
	"testing"

	"github.com/git-pkgs/roles"
)

type manifestVendorRoot struct {
	Path       string
	Ecosystem  string
	ConfigPath string
}

func TestManifestVendorRootTranslation(t *testing.T) {
	manifestRoots := []manifestVendorRoot{
		{Path: "node_modules", Ecosystem: npmEcosystem},
		{Path: "services/api/vendor", Ecosystem: golangEcosystem, ConfigPath: "services/api/vendor/modules.txt"},
		{Path: "tools/src/example/_vendor", Ecosystem: pypiEcosystem, ConfigPath: "tools/pyproject.toml"},
		{Path: "project/vendor", Ecosystem: cargoEcosystem, ConfigPath: "project/.cargo/config.toml"},
	}
	translated := make([]roles.VendorRoot, 0, len(manifestRoots))
	for _, root := range manifestRoots {
		translated = append(translated, roles.VendorRoot{
			Path: root.Path, Ecosystem: root.Ecosystem, EvidencePath: root.ConfigPath,
		})
	}
	want := []roles.VendorRoot{
		{Path: "node_modules", Ecosystem: npmEcosystem},
		{Path: "services/api/vendor", Ecosystem: golangEcosystem, EvidencePath: "services/api/vendor/modules.txt"},
		{Path: "tools/src/example/_vendor", Ecosystem: pypiEcosystem, EvidencePath: "tools/pyproject.toml"},
		{Path: "project/vendor", Ecosystem: cargoEcosystem, EvidencePath: "project/.cargo/config.toml"},
	}
	if !reflect.DeepEqual(translated, want) {
		t.Fatalf("translated roots = %#v, want %#v", translated, want)
	}
	classifier, err := roles.New(translated)
	if err != nil {
		t.Fatal(err)
	}
	for _, root := range want {
		result, err := classifier.Classify(root.Path + "/package/file.txt")
		if err != nil {
			t.Fatal(err)
		}
		matched := false
		for _, evidence := range result.Evidence {
			if evidence.Rule == contextVendorRule && evidence.Path == root.Path && evidence.Ecosystem == root.Ecosystem && evidence.EvidencePath == root.EvidencePath {
				matched = true
			}
		}
		if !matched {
			t.Errorf("missing translated evidence for %#v: %#v", root, result.Evidence)
		}
	}
}
