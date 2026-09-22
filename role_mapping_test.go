package roles

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"testing"
)

func TestRoleConstants(t *testing.T) {
	declared := declaredRoles(t)
	wantOrder := []Role{Source, Test, Fixture, Example, Benchmark, Fuzz, Vendor, Generated, BuildOutput, Cache, Documentation, Legal, Build, CI, Packaging, Tooling, Configuration, Minified}
	if !slices.Equal(roleOrder[:], wantOrder) {
		t.Fatalf("role order = %v, want %v", roleOrder, wantOrder)
	}
	if len(declared) != len(roleOrder) || len(declared) > 32 {
		t.Fatalf("role declarations=%d, ordered roles=%d", len(declared), len(roleOrder))
	}
	for i, role := range roleOrder {
		if !declared[role] {
			t.Fatalf("missing or duplicate role %q", role)
		}
		delete(declared, role)
		bit := roleBit(role)
		if bit != 1<<i || !bit.Has(role) || len(bit.List()) != 1 || bit.List()[0] != role {
			t.Fatalf("invalid mapping: %s", role)
		}
	}
	if roleBit("typo") != 0 || Set(^uint32(0)).Has("typo") {
		t.Fatal("unknown role matched")
	}
}

func declaredRoles(t *testing.T) map[Role]bool {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "roles.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	declared := map[Role]bool{}
	for _, decl := range file.Decls {
		group, ok := decl.(*ast.GenDecl)
		if !ok || group.Tok != token.CONST {
			continue
		}
		recordRoleConstants(t, group, declared)
	}

	return declared
}

func recordRoleConstants(t *testing.T, group *ast.GenDecl, declared map[Role]bool) {
	t.Helper()
	for _, spec := range group.Specs {
		value, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		typ, ok := value.Type.(*ast.Ident)
		if !ok || typ.Name != "Role" {
			continue
		}
		for _, expr := range value.Values {
			literal, ok := expr.(*ast.BasicLit)
			if !ok {
				t.Fatal("role must be a string literal")
			}
			name, err := strconv.Unquote(literal.Value)
			if err != nil {
				t.Fatal(err)
			}
			if declared[Role(name)] {
				t.Fatalf("duplicate role %q", name)
			}
			declared[Role(name)] = true
		}
	}
}
