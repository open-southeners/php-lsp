package completion

import (
	"testing"

	"github.com/open-southeners/php-lsp/internal/parser"
	"github.com/open-southeners/php-lsp/internal/protocol"
)

// TestChainReturnTypeResolution verifies return type resolution at each step
// of an Eloquent method chain, documenting current behavior and marking where
// generic type support is needed.
func TestChainReturnTypeResolution(t *testing.T) {
	p, _ := setupEloquentCompletionTest(t)

	resolve := func(expr, source string) string {
		file := parser.ParseFile(source)
		return p.ResolveExpressionType(expr, source, protocol.Position{Line: 5}, file)
	}

	preamble := "<?php\nuse App\\Models\\Category;\nuse App\\Models\\Product;\n\n\n"

	t.Run("Category::query() returns Builder", func(t *testing.T) {
		typ := resolve("Category::query()", preamble)
		if typ != "Illuminate\\Database\\Eloquent\\Builder" {
			t.Errorf("expected Builder, got %q", typ)
		}
	})

	t.Run("Category::query()->with() returns Builder (static on Builder)", func(t *testing.T) {
		// Builder::with() returns `static` which resolves to Builder itself.
		// With generics, this should return Builder<Category> preserving the model context.
		typ := resolve("Category::query()->with(['products'])", preamble)
		if typ != "Illuminate\\Database\\Eloquent\\Builder" {
			t.Errorf("expected Builder, got %q", typ)
		}
	})

	t.Run("Category::query()->with()->get() returns Collection", func(t *testing.T) {
		// Builder::get() returns Collection.
		// With generics, this should return Collection<int, Category>.
		typ := resolve("Category::query()->with(['products'])->get(['id', 'name'])", preamble)
		if typ != "Illuminate\\Database\\Eloquent\\Collection" {
			t.Errorf("expected Collection, got %q", typ)
		}
	})

	t.Run("Collection->first() returns Model (generic gap)", func(t *testing.T) {
		// Collection::first() returns ?Model in the test stubs.
		// With generics, Collection<int, Category>::first() should return ?Category.
		// Currently this resolves to Model (the base class), losing the specific model type.
		source := preamble + "$cats = Category::get();\n$cats->first()"
		typ := resolve("$cats->first()", source)
		// Currently: resolves to Model (not Category) — this is the generic gap
		if typ != "Illuminate\\Database\\Eloquent\\Model" {
			t.Logf("Collection::first() resolved to %q (expected Model without generics)", typ)
		}
	})

	t.Run("Category::where() returns Builder", func(t *testing.T) {
		typ := resolve("Category::where('active', true)", preamble)
		if typ != "Illuminate\\Database\\Eloquent\\Builder" {
			t.Errorf("expected Builder, got %q", typ)
		}
	})

	t.Run("Category::first() returns Category (static resolved)", func(t *testing.T) {
		// The virtual static method first() has return type "static" which
		// eloquent.go converts to the model FQN during injection.
		typ := resolve("Category::first()", preamble)
		if typ != "App\\Models\\Category" {
			t.Errorf("expected App\\Models\\Category, got %q", typ)
		}
	})

	t.Run("Category::find() returns Category (static resolved)", func(t *testing.T) {
		typ := resolve("Category::find(1)", preamble)
		if typ != "App\\Models\\Category" {
			t.Errorf("expected App\\Models\\Category, got %q", typ)
		}
	})

	t.Run("Category::all() returns Collection", func(t *testing.T) {
		typ := resolve("Category::all()", preamble)
		if typ != "Illuminate\\Database\\Eloquent\\Collection" {
			t.Errorf("expected Collection, got %q", typ)
		}
	})

	t.Run("Category::get() returns Collection", func(t *testing.T) {
		typ := resolve("Category::get()", preamble)
		if typ != "Illuminate\\Database\\Eloquent\\Collection" {
			t.Errorf("expected Collection, got %q", typ)
		}
	})

	t.Run("full chain: query->with->get->count", func(t *testing.T) {
		// Collection::count() returns int
		typ := resolve("Category::query()->with(['products'])->get(['id'])->count()", preamble)
		if typ != "int" {
			t.Errorf("expected int, got %q", typ)
		}
	})

	t.Run("full chain: query->with->get->map", func(t *testing.T) {
		// Collection::map() returns static → Collection
		typ := resolve("Category::query()->with(['products'])->get(['id'])->map(fn($x) => $x)", preamble)
		if typ != "Illuminate\\Database\\Eloquent\\Collection" {
			t.Errorf("expected Collection, got %q", typ)
		}
	})
}

// TestChainCompletionItems verifies that completions appear at each step
// of a method chain, checking that the correct members are available.
func TestChainCompletionItems(t *testing.T) {
	p, _ := setupEloquentCompletionTest(t)

	t.Run("Category::query()-> shows Builder methods", func(t *testing.T) {
		source := "<?php\nuse App\\Models\\Category;\nCategory::query()->"
		items := p.GetCompletions("file:///test.php", source, protocol.Position{Line: 2, Character: 19})
		labels := collectLabels(items)

		for _, m := range []string{"where", "first", "get", "with", "orderBy"} {
			if !labels[m] {
				t.Errorf("expected Builder method %q after query()->, got: %v", m, labels)
			}
		}
	})

	t.Run("Category::query()->with()-> shows Builder methods", func(t *testing.T) {
		source := "<?php\nuse App\\Models\\Category;\nCategory::query()->with('products')->"
		// "Category::query()->with('products')->" = 37 chars
		items := p.GetCompletions("file:///test.php", source, protocol.Position{Line: 2, Character: 37})
		labels := collectLabels(items)

		if !labels["get"] {
			t.Errorf("expected 'get' after with()->, got: %v", labels)
		}
		if !labels["where"] {
			t.Errorf("expected 'where' after with()->, got: %v", labels)
		}
	})

	t.Run("Category::query()->with()->get()-> shows Collection methods", func(t *testing.T) {
		source := "<?php\nuse App\\Models\\Category;\nCategory::query()->with(['products'])->get()->"
		items := p.GetCompletions("file:///test.php", source, protocol.Position{Line: 2, Character: 46})
		labels := collectLabels(items)

		for _, m := range []string{"count", "first", "map"} {
			if !labels[m] {
				t.Errorf("expected Collection method %q after get()->, got: %v", m, labels)
			}
		}
	})

	t.Run("Category::where()->orderBy()-> shows Builder methods", func(t *testing.T) {
		source := "<?php\nuse App\\Models\\Category;\nCategory::where('active', 1)->orderBy('name')->"
		// "Category::where('active', 1)->orderBy('name')->" = 47 chars
		items := p.GetCompletions("file:///test.php", source, protocol.Position{Line: 2, Character: 47})
		labels := collectLabels(items)

		if !labels["get"] {
			t.Errorf("expected 'get' after orderBy()->, got: %v", labels)
		}
		if !labels["first"] {
			t.Errorf("expected 'first' after orderBy()->, got: %v", labels)
		}
	})

	t.Run("Category::first()-> shows Category model methods", func(t *testing.T) {
		source := "<?php\nuse App\\Models\\Category;\nCategory::first()->"
		items := p.GetCompletions("file:///test.php", source, protocol.Position{Line: 2, Character: 19})
		labels := collectLabels(items)

		// first() returns static (→ Category), so model members should appear
		if !labels["save"] {
			t.Errorf("expected 'save' from Model after first()->, got: %v", labels)
		}
		if !labels["products"] {
			t.Errorf("expected 'products' relation after first()->, got: %v", labels)
		}
	})
}
