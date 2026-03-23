package completion

import (
	"strings"
	"testing"

	"github.com/open-southeners/tusk-php/internal/parser"
	"github.com/open-southeners/tusk-php/internal/protocol"
	"github.com/open-southeners/tusk-php/internal/resolve"
	"github.com/open-southeners/tusk-php/internal/symbols"
)

func setupTemplateProviderCoverage(t *testing.T) (*Provider, *symbols.Index, string, *parser.FileNode) {
	t.Helper()

	source := `<?php
namespace App;

class ShapeCarrier {}

class Arr {
    /**
     * @template TKey
     * @template TValue
     * @param array<TKey, TValue> $array
     * @return TValue|null
     */
    public static function first($array) {}
}

$rows = [
    ['id' => 1],
    ['id' => 2],
];

$selected = Arr::first($rows);
`

	idx := symbols.NewIndex()
	idx.IndexFile("file:///provider.php", source)
	file := parser.ParseFile(source)
	if file == nil {
		t.Fatal("expected parsed file")
	}
	return NewProvider(idx, nil, "none"), idx, source, file
}

func providerLineContaining(t *testing.T, source, needle string) int {
	t.Helper()
	for i, line := range strings.Split(source, "\n") {
		if strings.Contains(line, needle) {
			return i
		}
	}
	t.Fatalf("line containing %q not found", needle)
	return -1
}

func TestProviderGenericCoverageHelpers(t *testing.T) {
	p, idx, source, file := setupTemplateProviderCoverage(t)

	if p.resolver.TypedChainResolver == nil {
		t.Fatal("expected NewProvider to wire TypedChainResolver")
	}

	if got := p.ResolveExpressionTypeTyped("   ", source, protocol.Position{}, file); !got.IsEmpty() {
		t.Fatalf("expected empty expression type, got %#v", got)
	}

	if got := p.ResolveChainTypeTyped(source, "$rows", "->", protocol.Position{}, file); !got.IsEmpty() {
		t.Fatalf("expected empty chain type, got %#v", got)
	}

	if !hasUnresolvedTemplateParam(resolve.ResolvedType{FQN: "Collection", Params: []resolve.ResolvedType{{FQN: "TValue"}}}) {
		t.Fatal("expected unresolved template param in nested type")
	}
	if hasUnresolvedTemplateParam(resolve.ResolvedType{FQN: "Collection", Params: []resolve.ResolvedType{{FQN: "int"}}}) {
		t.Fatal("did not expect unresolved template param")
	}

	member := idx.Lookup("App\\Arr::first")
	if member == nil {
		t.Fatal("expected App\\Arr::first symbol")
	}

	pos := protocol.Position{Line: providerLineContaining(t, source, "$selected =")}

	rt := p.resolveMethodTemplateFromArgs(member, "[1, 2, 3]", source, pos, file, "")
	if rt.String() != "?int" {
		t.Fatalf("literal array inference got %q", rt.String())
	}

	rt = p.resolveMethodTemplateFromArgs(member, "$rows", source, pos, file, "")
	if rt.String() != "?array{id: int}" {
		t.Fatalf("variable array inference got %q", rt.String())
	}

	if rt := p.resolveMethodTemplateFromArgs(member, "", source, pos, file, ""); !rt.IsEmpty() {
		t.Fatalf("expected empty result for empty arg string, got %#v", rt)
	}
}
