package completion

import (
	"testing"

	"github.com/open-southeners/php-lsp/internal/protocol"
	"github.com/open-southeners/php-lsp/internal/symbols"
)

func TestSortPriority(t *testing.T) {
	tests := []struct {
		name     string
		sym      *symbols.Symbol
		ns       string
		wantPfx  string
	}{
		{"same namespace project", &symbols.Symbol{Name: "Foo", FQN: "App\\Models\\Foo", Source: symbols.SourceProject}, "App\\Models", "1"},
		{"different namespace project", &symbols.Symbol{Name: "Bar", FQN: "App\\Services\\Bar", Source: symbols.SourceProject}, "App\\Models", "2"},
		{"builtin", &symbols.Symbol{Name: "strlen", FQN: "strlen", Source: symbols.SourceBuiltin}, "App\\Models", "3"},
		{"vendor", &symbols.Symbol{Name: "collect", FQN: "collect", Source: symbols.SourceVendor}, "App\\Models", "4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortPriority(tt.sym, tt.ns)
			if got[0:1] != tt.wantPfx {
				t.Errorf("sortPriority() prefix = %q, want %q", got[0:1], tt.wantPfx)
			}
		})
	}
}

func TestSortOrdering(t *testing.T) {
	projectSame := sortPriority(&symbols.Symbol{Name: "Foo", FQN: "App\\Models\\Foo", Source: symbols.SourceProject}, "App\\Models")
	projectOther := sortPriority(&symbols.Symbol{Name: "Bar", FQN: "App\\Services\\Bar", Source: symbols.SourceProject}, "App\\Models")
	builtin := sortPriority(&symbols.Symbol{Name: "strlen", FQN: "strlen", Source: symbols.SourceBuiltin}, "App\\Models")
	vendor := sortPriority(&symbols.Symbol{Name: "collect", FQN: "collect", Source: symbols.SourceVendor}, "App\\Models")

	if projectSame >= projectOther {
		t.Error("same-namespace project should sort before other-namespace project")
	}
	if projectOther >= builtin {
		t.Error("project should sort before builtins")
	}
	if builtin >= vendor {
		t.Error("builtins should sort before vendor")
	}
}

func TestKeywordSortLast(t *testing.T) {
	idx := symbols.NewIndex()
	idx.RegisterBuiltins()
	p := NewProvider(idx, nil, "")

	source := "<?php\nnamespace App;\n"
	items := p.GetCompletions("file:///test.php", source, protocol.Position{Line: 2, Character: 0})

	for _, item := range items {
		if item.Kind == protocol.CompletionItemKindKeyword {
			if item.SortText[0:1] != "5" {
				t.Errorf("keyword %q has SortText %q, expected prefix '5'", item.Label, item.SortText)
			}
		}
	}
}

func TestExtractNamespace(t *testing.T) {
	tests := []struct {
		source string
		want   string
	}{
		{"<?php\nnamespace App\\Models;\n", "App\\Models"},
		{"<?php\nnamespace App\\Services;\nclass Foo {}", "App\\Services"},
		{"<?php\n// no namespace\n", ""},
	}
	for _, tt := range tests {
		got := extractNamespace(tt.source)
		if got != tt.want {
			t.Errorf("extractNamespace() = %q, want %q", got, tt.want)
		}
	}
}

func TestCompletionSourceSorting(t *testing.T) {
	idx := symbols.NewIndex()
	idx.RegisterBuiltins()

	// Index a project function
	idx.IndexFileWithSource("file:///project.php", `<?php
namespace App;
function str_project(): string { return ""; }
`, symbols.SourceProject)

	// Index a vendor function
	idx.IndexFileWithSource("file:///vendor.php", `<?php
function str_vendor(): string { return ""; }
`, symbols.SourceVendor)

	p := NewProvider(idx, nil, "")
	source := "<?php\nnamespace App;\nstr"
	items := p.GetCompletions("file:///test.php", source, protocol.Position{Line: 2, Character: 3})

	sortTexts := make(map[string]string)
	for _, item := range items {
		sortTexts[item.Label] = item.SortText
	}

	// Project symbol in same namespace should sort first
	if st, ok := sortTexts["str_project"]; ok {
		if st[0:1] != "1" {
			t.Errorf("str_project SortText = %q, expected prefix '1' (same namespace)", st)
		}
	}

	// Builtin str_contains should sort as builtin
	if st, ok := sortTexts["str_contains"]; ok {
		if st[0:1] != "3" {
			t.Errorf("str_contains SortText = %q, expected prefix '3' (builtin)", st)
		}
	}

	// Vendor function should sort last among symbols
	if st, ok := sortTexts["str_vendor"]; ok {
		if st[0:1] != "4" {
			t.Errorf("str_vendor SortText = %q, expected prefix '4' (vendor)", st)
		}
	}
}
