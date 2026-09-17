package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"runtime/debug"
	"strings"
	"unicode/utf8"

	"github.com/git-pkgs/roles"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("roles", flag.ContinueOnError)
	root := flags.String("root", "", "Walk a repository directory without reading file contents")
	labelsOnly := flags.Bool("labels-only", false, "Emit roles without collecting evidence")
	showVersion := flags.Bool("version", false, "Print module and corpus versions")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		return printVersion(output, *root, *labelsOnly, flags.NArg())
	}
	if (*root == "") == (flags.NArg() == 0) {
		return fmt.Errorf("provide paths or -root directory")
	}
	records := recordEncoder{encoder: json.NewEncoder(output)}
	if *root != "" {
		return runTree(*root, *labelsOnly, records)
	}
	return runPaths(flags.Args(), *labelsOnly, records)
}

type recordEncoder struct{ encoder *json.Encoder }

func (e recordEncoder) result(name string, result roles.Result) error {
	if err := validateJSONPath(name); err != nil {
		return err
	}
	return e.encoder.Encode(struct {
		Path          string `json:"path"`
		CorpusVersion string `json:"corpus_version"`
		roles.Result
	}{name, roles.CorpusVersion, result})
}

func (e recordEncoder) labels(name string, labels roles.Set) error {
	if err := validateJSONPath(name); err != nil {
		return err
	}
	return e.encoder.Encode(struct {
		Path          string       `json:"path"`
		CorpusVersion string       `json:"corpus_version"`
		Roles         []roles.Role `json:"roles"`
	}{name, roles.CorpusVersion, labels.List()})
}

func validateJSONPath(name string) error {
	if !utf8.ValidString(name) {
		return fmt.Errorf("JSON output requires UTF-8 paths: %q", name)
	}
	return nil
}

func printVersion(output io.Writer, root string, labelsOnly bool, paths int) error {
	if root != "" || labelsOnly || paths != 0 {
		return errors.New("-version cannot be combined with paths or other options")
	}
	_, err := fmt.Fprintf(output, "roles %s corpus %s\n", moduleVersion(), roles.CorpusVersion)
	return err
}

func runPaths(paths []string, labelsOnly bool, records recordEncoder) error {
	for _, name := range paths {
		if err := runPath(name, labelsOnly, records); err != nil {
			return fmt.Errorf("%q: %w", name, err)
		}
	}
	return nil
}

func runPath(name string, labelsOnly bool, records recordEncoder) error {
	if labelsOnly {
		labels, err := roles.Match(name)
		if err != nil {
			return err
		}
		return records.labels(name, labels)
	}
	result, err := roles.Classify(name)
	if err != nil {
		return err
	}
	return records.result(name, result)
}

func runTree(root string, labelsOnly bool, records recordEncoder) error {
	tree, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	if labelsOnly {
		err = roles.WalkMatch(tree.FS(), roles.WalkOptions{}, func(name string, labels roles.Set) error {
			return emitTreeEntry(name, func() error { return records.labels(name, labels) })
		})
	} else {
		err = roles.Walk(tree.FS(), roles.WalkOptions{}, func(name string, result roles.Result) error {
			return emitTreeEntry(name, func() error { return records.result(name, result) })
		})
	}
	return errors.Join(err, tree.Close())
}

func emitTreeEntry(name string, emit func() error) error {
	if !isGitMetadata(name) {
		return emit()
	}
	if strings.HasSuffix(name, "/") {
		return fs.SkipDir
	}
	return nil
}

func isGitMetadata(name string) bool {
	name = strings.TrimSuffix(name, "/")
	return name == ".git" || strings.HasSuffix(name, "/.git")
}

func moduleVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" {
		return "(devel)"
	}
	return info.Main.Version
}
