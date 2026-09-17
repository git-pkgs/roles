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

func TestRepositoryRoleSummaries(t *testing.T) {
	wantRevision := map[string]string{
		"brief":      "45d9c023ac4c64ab372c96fdf550fe8fc3828299",
		"git-pkgs":   "b77557b2ea522373c6982d70319e2677c2c0eb15",
		"scrutineer": "5ad2901b03d706859df7130217e73634c5a2508e",
	}
	wantCounts := map[string]string{
		"brief":      `{"build":5,"ci":3,"configuration":23,"documentation":3,"example":4,"fixture":106,"generated":3,"legal":2,"packaging":23,"source":29,"test":128}`,
		"git-pkgs":   `{"ci":2,"configuration":3,"documentation":14,"fixture":9,"legal":7,"source":155,"test":79,"tooling":3}`,
		"scrutineer": `{"ci":5,"configuration":4,"documentation":44,"fixture":21,"generated":8,"legal":2,"source":421,"test":193,"tooling":18,"vendor":9}`,
	}
	for _, repo := range loadRepositories(t) {
		if repo.Revision != wantRevision[repo.Name] {
			t.Fatalf("%s revision = %q, want %q", repo.Name, repo.Revision, wantRevision[repo.Name])
		}
		counts := map[roles.Role]int{}
		for _, name := range repo.Paths {
			set, err := roles.Match(name)
			if err != nil {
				t.Fatal(err)
			}
			for _, role := range set.List() {
				counts[role]++
			}
		}
		encoded, err := json.Marshal(counts)
		if err != nil {
			t.Fatal(err)
		}
		if string(encoded) != wantCounts[repo.Name] {
			t.Fatalf("%s role counts = %s", repo.Name, encoded)
		}
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
