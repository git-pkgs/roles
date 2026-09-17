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
independent evidence. Classification results use empty role and evidence
slices rather than nil slices.

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
| `src/Form.Designer.cs` | `source`, `generated` |
| `.pytest_cache/v/cache/nodeids` | `generated`, `cache` |
| `CMakeFiles/app.dir/main.o` | `generated`, `build-output` |
| `Pods/library/main.swift` | `vendor` |
| `gradlew` | `build` |
| `.gitattributes` | `configuration` |
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
`vendor`, `generated`, `build-output`, `cache`, `documentation`, `legal`,
`build`, `ci`, `packaging`, `tooling` and `configuration`. `build` identifies
build instructions and wrappers. `build-output` identifies specific emitted
build-system files and directories. These labels describe path conventions;
they do not establish that a tracked file is safe to delete. Coverage is based
on explicit conventions, not a complete inventory of every ecosystem.

Evidence identifies the rule, role, matched path and source, with a subtype or
ecosystem where applicable. Caller-supplied context can also identify the
repository path that supplied the evidence. Legal matches distinguish licence
and notice files. Roles follow a fixed order; evidence follows ancestor order
and then filename matches. A `source` role describes layout and does not
establish compilability or authorship.

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

Scanners that already enumerate files, such as brief, can call `Match` on
each accepted repository-relative path during their existing scan. This adds
no file reads or evidence allocations and does not require a second traversal.
`Match` supports concurrent calls; content analysis and directory exclusion
policies remain with the caller.

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

The classifier copies this context and supports concurrent use. Several
ecosystems or evidence paths can describe the same root; exact duplicates are
ignored, and evidence order is deterministic. The `context.vendor-root`
evidence uses `context` as its source and records the caller-supplied path
separately.
`LegalFileName` and `IsLegalDirectory` expose the same legal-name corpus for
consumers that already traverse paths themselves. `LegalFileNameBytes` and
`IsLegalDirectoryBytes` accept raw Git tree entry names without allocating for
ASCII corpus names.

Git tree walkers can classify components without rebuilding full paths. A
state is a value, so one parent can produce independent child states:

```go
state := classifier.RootState()
sourceState, err := state.Enter([]byte("src"))
if err != nil {
    return err
}
labels, err := sourceState.Match([]byte("main.go"))
```

`Enter` accepts one directory component and `Match` accepts one file
component. Both accept raw non-NUL Git filename bytes. `Roles` returns the
directory roles accumulated by a state. `Key` returns a comparable value that
captures the corpus version, configured vendor paths, inherited roles and
partial path-rule progress. A history walker can combine that key with a tree
object ID when caching classified subtrees. Evidence for selected occurrences
can be recovered later with `Classify` and the full repository-relative path.

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

Use `WalkMatch` when the visitor only needs labels. It reuses inherited role
sets without collecting or copying evidence, which reduces allocation for
deep trees. It has the same traversal order, limits and callback error handling
as `Walk`, and also has a classifier method for vendor-root context.

```go
err := roles.WalkMatch(tree, roles.WalkOptions{}, func(path string, set roles.Set) error {
    if set.Has(roles.CI) {
        fmt.Println(path)
    }
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

Other generated-file conventions include Python bytecode and tool caches,
CMake metadata, TypeScript build information, Flutter plugin registrants
and GCC coverage files. Specific caches such as `.pytest_cache/`,
`.parcel-cache/` and `.turbo/` receive `cache`. Specific output such as
`CMakeFiles/`, `.next/`, `.nuxt/` and `.svelte-kit/` receives `build-output`.
These roles are additive with `generated`. Generic names such as `.cache/`,
`build/`, `dist/` and `out/`, and virtual environments, receive no additional
role. Bower, JSPM and vcpkg dependency directories receive `vendor` alongside
any roles matched inside them.

Named minified JavaScript/CSS files, source maps and .NET designer files
also receive `generated`. Source-map and designer suffixes accept mixed
case; minified suffixes use lowercase `.min.js`, `-min.js`, `.min.css` and
`-min.css`. A plain `.d.ts` filename or a name such as `jquery.js` does not
establish generated or vendored content.

Named community documents include support, governance, maintainers,
authors and roadmaps, using case-insensitive stems and selected document
extensions, including `.markdown` and `.rdoc`. For example, `SUPPORT.md`,
`support.md` and `Support.md` match, while `support.py` and `SUPPORT.md.bak`
do not. Rakefile matching accepts any casing with an optional `.rb` extension.
Citation files, API/manual directories and installation/change documents
provide further documentation conventions. Bare `INSTALL`, `CHANGE` and
`CHANGES` match; their lowercase command names require a document extension.

Sources include legal-name matching from [licenses](https://github.com/git-pkgs/licenses),
layout and tool conventions from [brief](https://github.com/git-pkgs/brief),
manifest, lockfile and dependency-output names from
[manifests](https://github.com/git-pkgs/manifests), and selected generated-file
and dependency-directory conventions from [GitHub's gitignore templates](https://github.com/github/gitignore).
Tool-specific output directories also come from
[outline](https://github.com/git-pkgs/outline).
[GitHub Linguist](https://github.com/github-linguist/linguist) supplies additional
vendor, documentation and generated-file conventions, adapted to these roles.
[NOTICE](NOTICE) records source revisions and attribution. `CorpusVersion`
identifies the embedded classification semantics for caller caches.
Tests bind each version to a fingerprint of the canonical rules and their
classification output, so semantic changes require a new version entry.

Coverage differs from [GitHub Linguist](https://github.com/github-linguist/linguist):
roles treats `testdata` as test/fixture rather than vendor. Generic output and
cache names such as `dist` and `cache` receive no directory role.

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
go run ./cmd/roles -labels-only -root .
go run ./cmd/roles -version
```

Each JSONL record includes `corpus_version`. Full records contain `roles` and
`evidence` arrays, including empty arrays for unmatched paths. `-labels-only`
uses `Match` or `WalkMatch` and omits the evidence field. JSON encoding still
allocates output records; library callers can use `Set` directly. Tree mode
prunes Git metadata directories and `.git` files. Library traversal applies no
automatic directory exclusions.

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
checks and filesystem traversal. `BenchmarkWalkDisk` and `BenchmarkWalkMatchDisk`
compare evidence-producing and label-only walks over the same wide, monorepo
and deep layouts. `BenchmarkMillionPaths` measures a million
synthetic monorepo paths; repository inventories provide additional path
workloads. These measurements exclude blob I/O and are not a throughput
claim for a complete repository scan.

## License

Released under the [MIT License](LICENSE). See [NOTICE](NOTICE) for corpus
source attribution and modification details, including conventions adapted
from GitHub's CC0-1.0 gitignore templates.
