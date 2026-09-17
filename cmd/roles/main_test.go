package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/git-pkgs/roles"
)

const (
	labelsOnlyFlag = "-labels-only"
	rootFlag       = "-root"
	versionFlag    = "-version"
)

func TestRun(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"vendor/sqlite/LICENSE"}, &out); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Path          string
		CorpusVersion string `json:"corpus_version"`
		roles.Result
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Path != "vendor/sqlite/LICENSE" || got.CorpusVersion != roles.CorpusVersion || !got.Has(roles.Vendor) || !got.Has(roles.Legal) {
		t.Fatal(got)
	}
}

func TestRunJSONSchema(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"LICENSE"}, &out); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("{\"path\":\"LICENSE\",\"corpus_version\":%q,\"roles\":[\"legal\"],\"evidence\":[{\"rule\":\"legal.prefix-fold.license\",\"role\":\"legal\",\"path\":\"LICENSE\",\"subtype\":\"license\",\"source\":\"licenses\"}]}\n", roles.CorpusVersion)
	if out.String() != want {
		t.Fatalf("JSON = %s, want %s", out.String(), want)
	}
	out.Reset()
	if err := run([]string{labelsOnlyFlag, "LICENSE"}, &out); err != nil {
		t.Fatal(err)
	}
	want = fmt.Sprintf("{\"path\":\"LICENSE\",\"corpus_version\":%q,\"roles\":[\"legal\"]}\n", roles.CorpusVersion)
	if out.String() != want {
		t.Fatalf("labels JSON = %s, want %s", out.String(), want)
	}
}

func TestRunTree(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "testdata"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "testdata", "package-lock.json"), []byte(`{"lockfileVersion":3}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../outside", filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := run([]string{rootFlag, root}, &out); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(&out)
	var dir, file struct {
		Path          string
		CorpusVersion string `json:"corpus_version"`
		roles.Result
	}
	if err := decoder.Decode(&dir); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(&file); err != nil {
		t.Fatal(err)
	}
	if dir.Path != "testdata/" || file.Path != "testdata/package-lock.json" || dir.CorpusVersion != roles.CorpusVersion || file.CorpusVersion != roles.CorpusVersion || !file.Has(roles.Generated) || !file.Has(roles.Fixture) {
		t.Fatal(dir, file)
	}
}

func TestRunTreeSkipsGitMetadata(t *testing.T) {
	const (
		githubWorkflow = ".github/workflows/ci.yml"
		projectGitFile = "project.git/file"
		sourceFile     = "src/main.go"
	)
	root := t.TempDir()
	for _, name := range []string{
		".git/objects/object",
		"nested/.git/config",
		"submodule/.git",
		githubWorkflow,
		projectGitFile,
		sourceFile,
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{rootFlag, root}, {labelsOnlyFlag, rootFlag, root}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			paths := runTreePaths(t, args)
			for _, path := range paths {
				clean := strings.TrimSuffix(path, "/")
				if clean == ".git" || strings.HasPrefix(clean, ".git/") || strings.Contains(clean, "/.git/") || strings.HasSuffix(clean, "/.git") {
					t.Fatalf("emitted Git metadata path %q", path)
				}
			}
			for _, name := range []string{githubWorkflow, projectGitFile, sourceFile} {
				if !slices.Contains(paths, name) {
					t.Errorf("missing non-metadata path %q", name)
				}
			}
		})
	}
}

func runTreePaths(t *testing.T, args []string) []string {
	t.Helper()
	var out bytes.Buffer
	if err := run(args, &out); err != nil {
		t.Fatal(err)
	}
	var paths []string
	decoder := json.NewDecoder(&out)
	for decoder.More() {
		var got struct {
			Path          string
			CorpusVersion string `json:"corpus_version"`
			roles.Result
		}
		if err := decoder.Decode(&got); err != nil {
			t.Fatal(err)
		}
		if got.CorpusVersion != roles.CorpusVersion {
			t.Fatalf("corpus version = %q", got.CorpusVersion)
		}
		paths = append(paths, got.Path)
	}
	return paths
}

func TestRunInvalidUTF8(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"src/\xff.go"}, &out); err == nil || out.Len() != 0 {
		t.Fatal("invalid path was silently replaced in JSON")
	}
}

func TestRunLabelsOnly(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "vendor"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "vendor", "LICENSE"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{labelsOnlyFlag, "vendor/LICENSE"},
		{labelsOnlyFlag, rootFlag, root},
	} {
		var out bytes.Buffer
		if err := run(args, &out); err != nil {
			t.Fatal(err)
		}
		decoder := json.NewDecoder(&out)
		found := false
		for decoder.More() {
			var got struct {
				Path          string
				CorpusVersion string          `json:"corpus_version"`
				Roles         []roles.Role    `json:"roles"`
				Evidence      json.RawMessage `json:"evidence"`
			}
			if err := decoder.Decode(&got); err != nil {
				t.Fatal(err)
			}
			if got.CorpusVersion != roles.CorpusVersion || got.Roles == nil || got.Evidence != nil {
				t.Fatal("unexpected evidence", got)
			}
			if got.Path == "vendor/LICENSE" {
				found = true
				if !slices.Contains(got.Roles, roles.Vendor) || !slices.Contains(got.Roles, roles.Legal) || len(got.Roles) != 2 {
					t.Fatal(got)
				}
			}
		}
		if !found {
			t.Fatal("missing file result")
		}
	}
}

func TestRunEmptyResultJSON(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"main.go"}, &out); err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if string(got["roles"]) != "[]" || string(got["evidence"]) != "[]" {
		t.Fatalf("empty result = %s", out.Bytes())
	}
	var version string
	if err := json.Unmarshal(got["corpus_version"], &version); err != nil || version != roles.CorpusVersion {
		t.Fatalf("corpus version = %q, %v", version, err)
	}
}

func TestRunVersion(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{versionFlag}, &out); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("roles %s corpus %s\n", moduleVersion(), roles.CorpusVersion)
	if out.String() != want {
		t.Fatalf("version = %q, want %q", out.String(), want)
	}
	for _, args := range [][]string{{versionFlag, "main.go"}, {versionFlag, labelsOnlyFlag}} {
		out.Reset()
		if err := run(args, &out); err == nil || out.Len() != 0 {
			t.Fatalf("accepted version arguments %v", args)
		}
	}
}
