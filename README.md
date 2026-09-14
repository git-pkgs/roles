# roles

Go library for classifying repository paths as source, tests, fixtures,
vendored code, generated files and other roles. Path classification reads
no file contents and requires no checkout.

The same contents under `src/sqlite3.c`, `vendor/sqlite/sqlite3.c` and
`test/fixtures/sqlite3.c` carry different context. Content analysis can run
once per unique blob, with roles describing each occurrence separately.

Results contain additive roles and optional evidence. Callers decide what
to scan, count or include in a report. The API and corpus are early and
may change.

For fixed inputs, corpus revision and context, classification is deterministic.
Tools can use these labels to select files before expensive scans or LLM
calls, without asking a model to infer roles on every run.

## Installation

```sh
go get github.com/git-pkgs/roles
```

## Use cases

| Composition | What it enables |
| --- | --- |
| licenses + roles | Licence findings associated with legal files, source layouts and bundled code |
| manifests + roles | Dependency observations separated by project, fixture, example and vendor paths |
| brief/distill + roles | Repository summaries and scoped project classification |
| scrutineer + roles | Analysis routing and role context attached to security findings |

Search indexes can filter for C source excluding vendor/generated, CI YAML
or test-only Python. Summaries could describe a Go application with generated
protobufs and vendored C, while diff reports could distinguish generated-file
changes from changes to source files.

Scanners can select build scripts for execution review or newly vendored
trees for dependency investigation. Callers supply the filtering and ranking
criteria; a role does not establish ownership or whether a file is safe to skip.

## Classification

A path can have several roles, or none when it carries no recognized
convention. Directory matches apply to descendants; filename rules add
independent evidence.

| Path | Roles |
| --- | --- |
| `LICENSE` | `legal` |
| `LICENSES/MIT.txt` | `legal` |
| `vendor/sqlite/LICENSE` | `vendor`, `legal` |
| `src/parser.go` | `source` |
| `internal/parser_test.go` | `source`, `test` |
| `test/fixtures/input.json` | `test`, `fixture` |
| `testdata/package-lock.json` | `test`, `fixture`, `generated` |
| `packages/api/uv.lock` | `generated` |
| `examples/client/main.go` | `example` |
| `docs/api.md` | `documentation` |
| `.github/SECURITY.md` | `documentation` |
| `CODE_OF_CONDUCT.md` | `documentation` |
| `.github/workflows/test.yml` | `ci` |
| `packages/service/.gitlab-ci.yml` | `ci` |
| `vendor/component/Makefile.in` | `vendor`, `build` |
| `test/fixtures/setup.py` | `test`, `fixture`, `packaging`, `configuration` |
| `.github/SUPPORT.md` | `documentation` |
| `main.go` | None |

The vocabulary is `source`, `test`, `fixture`, `example`, `benchmark`, `fuzz`,
`vendor`, `generated`, `documentation`, `legal`, `build`, `ci`, `packaging`,
`tooling` and `configuration`. Coverage is based on explicit conventions,
not a complete inventory of every ecosystem.

Evidence identifies the rule, role, matched path and origin, with a subtype
or ecosystem where applicable. Legal matches distinguish licence and notice
files. Roles follow a fixed order; evidence follows ancestor order and then
filename matches. A `source` role describes layout and does not establish
compilability or authorship.

## Usage

```go
result, err := roles.Classify("vendor/sqlite/LICENSE")
if err != nil {
    return err
}
for _, role := range result.Roles {
    fmt.Println(role)
}
```

Output:

```text
vendor
legal
```

For filtering without evidence allocation, use `Match`:

```go
labels, err := roles.Match("testdata/package-lock.json")
if err != nil {
    return err
}
if labels.Has(roles.Fixture) {
    // Apply the caller's fixture policy.
}
```

Paths use `/` separators. A trailing slash denotes a directory, so `vendor/`
is a vendor directory while `vendor` alone is a filename. Empty paths,
absolute paths, NUL bytes, repeated separators and `.` or `..` components
return `ErrInvalidPath`. Backslashes and invalid UTF-8 bytes are literal
filename bytes; the library does not normalize them.

Configured vendor roots can supply additional evidence:

```go
classifier, err := roles.New([]roles.VendorRoot{
    {Path: "deps/crates", Ecosystem: "cargo", EvidencePath: ".cargo/config.toml"},
})
if err != nil {
    return err
}
result, err := classifier.Classify("deps/crates/example/src/lib.rs")
```

The classifier copies this context and supports concurrent use. The
`context.vendor-root` evidence records caller-supplied information.
`LegalFileName` and `IsLegalDirectory` expose the same legal-name corpus for
consumers that already traverse paths themselves.

## Trees and content

`Walk` accepts an `fs.FS` and a callback, reusing directory matches as it
visits descendants. It emits directories with trailing slashes, skips
symlinks and special files, and propagates traversal and callback errors.
Callbacks may return `fs.SkipDir` or `fs.SkipAll` to control traversal.

```go
err := roles.Walk(tree, roles.WalkOptions{}, func(path string, result roles.Result) error {
    fmt.Println(path, result.Roles)
    return nil
})
```

The default limits are one million entries and 256 path components.
`WalkOptions` can change either limit; exceeding one returns `ErrLimit`,
with earlier callback results already delivered. Traversal is lexical and
buffers directory listings, so the entry limit is not a strict memory cap.
The caller supplies a rooted filesystem; `os.Root.FS` provides containment
for a directory on disk. No files are read for content classification during
`Walk`, and no directory names are automatically excluded.

`ClassifyBlob(path, contents)` adds optional generated Go header detection;
there is also a classifier method that retains vendor-root context. Supply
the complete file contents. Inspection is limited to the first 8 KiB and
40 lines, ending at the first non-comment token. Go tokenization prevents
markers in strings or block comments from becoming generated evidence.

The result includes `HeaderChecked`, `BytesExamined` and `HeaderLimited`.
Other file types receive path-only classification. Missing generated
evidence does not prove a file was handwritten, and a generated lockfile
may still be essential dependency evidence.

## Corpus

The embedded [rule corpus](corpus/rules.json) uses exact directory and
filename matches, case-insensitive stems with allowed extensions, bounded
prefixes and suffixes. It does not evaluate regexes or repository-supplied
rules. Legal names are case insensitive; most layout
names are case sensitive. Directory conventions apply at any depth,
including package directories in monorepos.

CI filenames cover GitLab CI, Buildkite, Jenkins, Azure Pipelines, Drone
and Travis CI alongside the GitHub Actions, CircleCI and `.buildkite` directories.
Build conventions include Make, CMake, Meson, Autotools, Rake, Gradle and
Maven. Packaging matches include Python setup files, Composer manifests
and Ruby gemspecs. These are filename conventions; file contents can use
those names for other purposes.

Generated dependency files include lockfiles for Bundler, Python tools,
Swift, Gradle, NuGet and other package managers. Selected dependency
reports such as `npm-ls.json` and `go.graph` have subtype
`dependency-report`; NuGet assets and `.deps.json` files use
`dependency-resolution`. There is no blanket `*.lock` rule.

Named community documents include support, governance, maintainers,
authors and roadmaps, using case-insensitive stems and selected document
extensions, including `.markdown` and `.rdoc`. For example, `SUPPORT.md`,
`support.md` and `Support.md` match, while `support.py` and `SUPPORT.md.bak`
do not. Rakefile matching accepts any casing with an optional `.rb` extension.

Sources include legal-name matching from [licenses](https://github.com/git-pkgs/licenses),
layout and tool conventions from [brief](https://github.com/git-pkgs/brief),
and manifest, lockfile and dependency-output names from
[manifests](https://github.com/git-pkgs/manifests).
[NOTICE](NOTICE) records source revisions and attribution. `CorpusVersion`
identifies the embedded classification semantics for caller caches.

Coverage differs from [GitHub Linguist](https://github.com/github-linguist/linguist):
roles treats `testdata` as test/fixture rather than vendor. Build-output and cache directories such as `dist` and `cache`
currently receive no directory role.

## Related tools

Potential consumers include [git-pkgs](https://github.com/git-pkgs/git-pkgs)
for dependency analysis, licenses and
[git-spdx](https://github.com/git-pkgs/git-spdx) for licence reporting, brief
for repository summaries, [distill](https://github.com/git-pkgs/distill) for project classification, and [outline](https://github.com/git-pkgs/outline)
for code context. [scrutineer](https://github.com/alpha-omega-security/scrutineer)
could attach roles to security findings. These integrations are not shipped.

[manifests](https://github.com/git-pkgs/manifests) retains package-manager
configuration parsing and vendor-root discovery;
[magic](https://github.com/git-pkgs/magic) detects formats and encodings.
Generated and vendored files may still require scanning, and a manifest's
path role does not change dependency scopes declared inside it.

Cache results with their path, corpus version and supplied context; a blob
ID or subtree hash alone is insufficient. Content observations that depend
on filenames or repository settings need those inputs in their cache keys
too. The library currently keeps no cross-tree cache.

## CLI

The development CLI emits one JSON record per path or tree entry. It rejects
non-UTF-8 paths rather than replacing their bytes in JSON:

```sh
go run ./cmd/roles vendor/sqlite/LICENSE testdata/package-lock.json
go run ./cmd/roles -root .
```

## Testing

Tests cover corpus examples, rule reachability, path validation, inherited
context, generated headers and CLI behaviour. Pinned path inventories from
three repositories check agreement between tree and individual-path
classification. These inventories are regression inputs, not independently
labelled ground truth.

```sh
go test -race ./...
go test -run '^$' -fuzz '^FuzzClassify$' -fuzztime=30s
go test -run '^$' -fuzz '^FuzzBlob$' -fuzztime=30s
go test -run '^$' -fuzz '^FuzzWalk$' -fuzztime=30s
```

## Benchmarks

```sh
go test -run '^$' -bench . -benchmem
go test -run '^$' -bench BenchmarkClassify -o /tmp/roles-profile.test -cpuprofile=/tmp/roles.cpu -memprofile=/tmp/roles.heap
go tool pprof /tmp/roles.cpu
```

Benchmarks separate label matching, evidence allocation, bounded content
checks and filesystem traversal. `BenchmarkMillionPaths` measures a million
synthetic monorepo paths; repository inventories provide additional path
workloads. These measurements exclude blob I/O and are not a throughput
claim for a complete repository scan.

## License

Released under the [MIT License](LICENSE). See [NOTICE](NOTICE) for corpus
source attribution and modification details.
