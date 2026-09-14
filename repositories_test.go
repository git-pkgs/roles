package roles_test

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/git-pkgs/roles"
)

type repositoryCorpus struct {
	Name     string
	Revision string
	Paths    []string
}

func loadRepositories(tb testing.TB) []repositoryCorpus {
	tb.Helper()
	data, err := os.ReadFile("testdata/repositories.json")
	if err != nil {
		tb.Fatal(err)
	}
	var repos []repositoryCorpus
	if err := json.Unmarshal(data, &repos); err != nil {
		tb.Fatal(err)
	}
	return repos
}

func TestRepositoryTrees(t *testing.T) {
	for _, repo := range loadRepositories(t) {
		t.Run(repo.Name, func(t *testing.T) {
			tree := fstest.MapFS{}
			for _, name := range repo.Paths {
				tree[name] = &fstest.MapFile{}
			}
			files := 0
			err := roles.Walk(tree, roles.WalkOptions{}, func(name string, got roles.Result) error {
				want, err := roles.Classify(name)
				if err != nil {
					return err
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("different tree evidence at %s", name)
				}
				if !strings.HasSuffix(name, "/") {
					files++
				}
				return nil
			})
			if err != nil || files != len(repo.Paths) {
				t.Fatalf("files=%d expected=%d error=%v", files, len(repo.Paths), err)
			}
		})
	}
}

func BenchmarkRepositories(b *testing.B) {
	for _, repo := range loadRepositories(b) {
		b.Run(repo.Name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				for _, name := range repo.Paths {
					_, _ = roles.Match(name)
				}
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*len(repo.Paths)), "ns/path")
		})
	}
}

func BenchmarkRepositoriesParallel(b *testing.B) {
	for _, repo := range loadRepositories(b) {
		b.Run(repo.Name, func(b *testing.B) {
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					_, _ = roles.Match(repo.Paths[i])
					i++
					if i == len(repo.Paths) {
						i = 0
					}
				}
			})
		})
	}
}
