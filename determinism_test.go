package roles_test

import (
	"reflect"
	"sync"
	"testing"

	"github.com/git-pkgs/roles"
)

func TestConcurrentDeterminism(t *testing.T) {
	roots := []roles.VendorRoot{
		{Path: cratesVendorRoot, Ecosystem: npmEcosystem, EvidencePath: packageJSON},
		{Path: cratesVendorRoot, Ecosystem: cargoEcosystem, EvidencePath: cargoConfig},
		{Path: cratesVendorRoot, Ecosystem: cargoEcosystem, EvidencePath: secondaryCargoConfig},
	}
	first, err := roles.New(roots)
	if err != nil {
		t.Fatal(err)
	}
	second, err := roles.New([]roles.VendorRoot{roots[2], roots[0], roots[1]})
	if err != nil {
		t.Fatal(err)
	}
	name := "deps/crates/src/parser_test.go"
	expected, err := first.Classify(name)
	if err != nil {
		t.Fatal(err)
	}
	for range 16 {
		t.Run("worker", func(t *testing.T) {
			t.Parallel()
			for range 100 {
				for _, classifier := range []*roles.Classifier{first, second} {
					got, err := classifier.Classify(name)
					if err != nil || !reflect.DeepEqual(got, expected) {
						t.Fatalf("non-deterministic result: %#v, %v", got, err)
					}
					set, err := classifier.Match(name)
					if err != nil || !reflect.DeepEqual(set.List(), expected.Roles) {
						t.Fatalf("non-deterministic labels: %v, %v", set, err)
					}
				}
			}
		})
	}
}

func TestDeterministicAcrossWorkerCounts(t *testing.T) {
	classifier, err := roles.New([]roles.VendorRoot{
		{Path: cratesVendorRoot, Ecosystem: cargoEcosystem, EvidencePath: cargoConfig},
		{Path: cratesVendorRoot, Ecosystem: npmEcosystem, EvidencePath: packageJSON},
	})
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{
		"deps/crates/src/parser_test.go",
		"packages/api/.github/workflows/ci.yml",
		"test/fixtures/project.tsbuildinfo",
		fingerprintNoticePath,
		mainGoPath,
	}
	want := classifyConcurrently(t, classifier, paths, 1)
	for _, workers := range []int{2, 8} {
		got := classifyConcurrently(t, classifier, paths, workers)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%d workers changed results: %#v", workers, got)
		}
	}
}

func classifyConcurrently(t *testing.T, classifier *roles.Classifier, paths []string, workers int) []roles.Result {
	t.Helper()
	results := make([]roles.Result, len(paths))
	errors := make([]error, workers)
	var group sync.WaitGroup
	for worker := range workers {
		group.Go(func() {
			for index := worker; index < len(paths); index += workers {
				result, err := classifier.Classify(paths[index])
				if err != nil {
					errors[worker] = err
					return
				}
				results[index] = result
			}
		})
	}
	group.Wait()
	for _, err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	return results
}
