package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
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
	if err := flags.Parse(args); err != nil {
		return err
	}
	if (*root == "") == (flags.NArg() == 0) {
		return fmt.Errorf("provide paths or -root directory")
	}
	encoder := json.NewEncoder(output)
	emit := func(name string, result roles.Result) error {
		if !utf8.ValidString(name) {
			return fmt.Errorf("JSON output requires UTF-8 paths: %q", name)
		}
		return encoder.Encode(struct {
			Path string `json:"path"`
			roles.Result
		}{name, result})
	}
	if *root != "" {
		tree, err := os.OpenRoot(*root)
		if err != nil {
			return err
		}
		err = roles.Walk(tree.FS(), roles.WalkOptions{}, emit)
		return errors.Join(err, tree.Close())
	}
	for _, name := range flags.Args() {
		result, err := roles.Classify(name)
		if err != nil {
			return fmt.Errorf("%q: %w", name, err)
		}
		if err := emit(name, result); err != nil {
			return err
		}
	}
	return nil
}
