package hover

import (
	"strings"
	"testing"

	"github.com/open-southeners/tusk-php/internal/parser"
	"github.com/open-southeners/tusk-php/internal/protocol"
	"github.com/open-southeners/tusk-php/internal/resolve"
	"github.com/open-southeners/tusk-php/internal/symbols"
)

func hoverLineContaining(t *testing.T, source, needle string) protocol.Position {
	t.Helper()
	for i, line := range strings.Split(source, "\n") {
		if strings.Contains(line, needle) {
			return protocol.Position{Line: i, Character: strings.Index(line, needle)}
		}
	}
	t.Fatalf("line containing %q not found", needle)
	return protocol.Position{}
}

func TestReplaceReturnTypeCoverage(t *testing.T) {
	docblockContent := "```php\nfunction fetch()\n```\n\n**Returns** `Collection`"
	replaced := replaceReturnType(docblockContent, "Collection<int, App\\User>")
	if !strings.Contains(replaced, "**Returns** `Collection<int, App\\User>`") {
		t.Fatalf("docblock replacement failed:\n%s", replaced)
	}

	inlineContent := "```php\npublic function fetch(): Collection\n```"
	replaced = replaceReturnType(inlineContent, "Collection<int, App\\User>")
	if !strings.Contains(replaced, "public function fetch(): Collection<int, App\\User>") {
		t.Fatalf("inline replacement failed:\n%s", replaced)
	}

	if got := replaceReturnType("no return marker here", "X"); got != "no return marker here" {
		t.Fatalf("unexpected no-op replacement: %q", got)
	}

	if got := joinShortNames([]string{"App\\Contracts\\Logger", "App\\Models\\User"}); got != "Logger, User" {
		t.Fatalf("got %q", got)
	}
}

func TestHoverGenericReturnTypeReplacement(t *testing.T) {
	source := `<?php
namespace App;

class User {}
class Collection {}

class Service {
    public function fetch(): Collection {
        return new Collection();
    }
}

$service = new Service();
$service->fetch();
`

	idx := symbols.NewIndex()
	idx.IndexFile("file:///hover.php", source)
	p := NewProvider(idx, nil, "none")
	p.GenericExprResolver = func(expr, source string, pos protocol.Position, file *parser.FileNode) resolve.ResolvedType {
		return resolve.ResolvedType{
			FQN:    "App\\Collection",
			Params: []resolve.ResolvedType{{FQN: "int"}, {FQN: "App\\User"}},
		}
	}

	pos := hoverLineContaining(t, source, "$service->fetch")
	pos.Character = strings.Index(strings.Split(source, "\n")[pos.Line], "fetch")
	hover := p.GetHover("file:///hover.php", source, pos)
	if hover == nil {
		t.Fatal("expected hover result")
	}
	if !strings.Contains(hover.Contents.Value, "App\\Collection<int, App\\User>") {
		t.Fatalf("expected generic return type in hover, got:\n%s", hover.Contents.Value)
	}
}

func TestHoverVariableShowsGenericType(t *testing.T) {
	p := NewProvider(symbols.NewIndex(), nil, "none")
	p.SetTypedChainResolver(func(expr, source string, pos protocol.Position, file *parser.FileNode) resolve.ResolvedType {
		if expr == "Factory::make()" {
			return resolve.ResolvedType{
				FQN:    "App\\Collection",
				Params: []resolve.ResolvedType{{FQN: "int"}, {FQN: "App\\User"}},
			}
		}
		return resolve.ResolvedType{}
	})

	source := "<?php\n$items = Factory::make();\n$items;\n"
	hover := p.GetHover("file:///vars.php", source, protocol.Position{Line: 2, Character: 2})
	if hover == nil {
		t.Fatal("expected hover result")
	}
	if !strings.Contains(hover.Contents.Value, "App\\Collection<int, App\\User> $items") {
		t.Fatalf("expected generic variable hover, got:\n%s", hover.Contents.Value)
	}
}

func TestResolveExpressionTypeCoverage(t *testing.T) {
	p, src := setupProvider(t)
	file := parser.ParseFile(src)
	if file == nil {
		t.Fatal("expected parsed file")
	}

	if got := p.resolveExpressionType("", src, protocol.Position{}, file); got != "" {
		t.Fatalf("expected empty type for empty expression, got %q", got)
	}

	pos := charPosOf(t, src, "info", "getLogger()->info")
	if got := p.resolveExpressionType("$handler->getLogger()", src, pos, file); got != "Monolog\\Logger" {
		t.Fatalf("got %q", got)
	}
}
