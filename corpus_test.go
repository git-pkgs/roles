package roles_test

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/git-pkgs/roles"
)

func TestEveryRuleIsReachable(t *testing.T) {
	data, err := os.ReadFile("corpus/rules.json")
	if err != nil {
		t.Fatal(err)
	}
	var rules []struct {
		ID, Kind, Pattern string
		Extensions        []string
	}
	if err := json.Unmarshal(data, &rules); err != nil {
		t.Fatal(err)
	}
	for _, rule := range rules {
		t.Run(rule.ID, func(t *testing.T) {
			name := rule.Pattern
			switch rule.Kind {
			case "directory", "directory-fold", "directory-path":
				name += "/arbitrary.bin"
			case "suffix", "suffix-fold":
				name = "file" + name
			case "stem-fold":
				name += rule.Extensions[0]
			}
			got, err := roles.Classify(name)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.ContainsFunc(got.Evidence, func(e roles.Evidence) bool { return e.Rule == rule.ID }) {
				t.Fatalf("unreachable rule %s", rule.ID)
			}
		})
	}
}

func TestManifestProvenance(t *testing.T) {
	for ecosystem, names := range map[string][]string{
		cargoEcosystem: {"Cargo.lock", "Cargo.toml"},
		npmEcosystem:   {"package-lock.json", "yarn.lock", "pnpm-lock.yaml", packageJSON},
		"composer":     {"composer.lock", "composer.json"},
		pypiEcosystem:  {"pyproject.toml", "setup.py", "setup.cfg"},
		"gem":          {"roles.gemspec"},
		"maven":        {"build.gradle", "build.gradle.kts", "pom.xml"},
	} {
		for _, name := range names {
			t.Run(name, func(t *testing.T) {
				path := "packages/service/" + name
				got, err := roles.Classify(path)
				if err != nil || len(got.Evidence) == 0 {
					t.Fatalf("Classify = %+v, %v", got, err)
				}
				for _, evidence := range got.Evidence {
					if evidence.Path != path || evidence.Source != "manifests" || evidence.Ecosystem != ecosystem || evidence.EvidencePath != "" {
						t.Errorf("unexpected evidence: %+v", evidence)
					}
				}
			})
		}
	}
}
