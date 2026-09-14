package roles_test

import (
	"reflect"
	"testing"

	"github.com/git-pkgs/roles"
)

func TestConcurrentDeterminism(t *testing.T) {
	roots := []roles.VendorRoot{
		{Path: "deps/crates", EvidencePath: cargoConfig},
		{Path: "other/vendor", EvidencePath: "other/.cargo/config.toml"},
	}
	first, err := roles.New(roots)
	if err != nil {
		t.Fatal(err)
	}
	second, err := roles.New([]roles.VendorRoot{roots[1], roots[0]})
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
				}
			}
		})
	}
}
