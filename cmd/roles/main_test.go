package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/git-pkgs/roles"
)

func TestRun(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"vendor/sqlite/LICENSE"}, &out); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Path string
		roles.Result
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Path != "vendor/sqlite/LICENSE" || !got.Has(roles.Vendor) || !got.Has(roles.Legal) {
		t.Fatal(got)
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
	if err := run([]string{"-root", root}, &out); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(&out)
	var dir, file struct {
		Path string
		roles.Result
	}
	if err := decoder.Decode(&dir); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(&file); err != nil {
		t.Fatal(err)
	}
	if dir.Path != "testdata/" || file.Path != "testdata/package-lock.json" || !file.Has(roles.Generated) || !file.Has(roles.Fixture) {
		t.Fatal(dir, file)
	}
}

func TestRunInvalidUTF8(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"src/\xff.go"}, &out); err == nil || out.Len() != 0 {
		t.Fatal("invalid path was silently replaced in JSON")
	}
}
