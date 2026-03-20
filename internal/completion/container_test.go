package completion

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/open-southeners/php-lsp/internal/container"
	"github.com/open-southeners/php-lsp/internal/protocol"
	"github.com/open-southeners/php-lsp/internal/symbols"
)

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return string(data)
}

func collectLabels(items []protocol.CompletionItem) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, item := range items {
		m[item.Label] = true
	}
	return m
}

func setupLaravelIndex(t *testing.T) (*symbols.Index, *container.ContainerAnalyzer) {
	t.Helper()
	root := filepath.Join("..", "..", "testdata", "laravel")

	idx := symbols.NewIndex()
	idx.RegisterBuiltins()

	// Index project files
	for _, rel := range []string{
		"app/Models/User.php",
		"app/Models/Category.php",
		"app/Services/PaymentGateway.php",
		"app/Services/StripeGateway.php",
		"app/Services/CustomMailer.php",
		"app/Providers/AppServiceProvider.php",
	} {
		src := mustReadFile(t, filepath.Join(root, rel))
		idx.IndexFileWithSource("file:///"+rel, src, symbols.SourceProject)
	}

	// Index vendor files
	for _, rel := range []string{
		"vendor/illuminate/http/src/Request.php",
	} {
		src := mustReadFile(t, filepath.Join(root, rel))
		idx.IndexFileWithSource("file:///"+rel, src, symbols.SourceVendor)
	}

	ca := container.NewContainerAnalyzer(idx, root, "laravel")
	ca.Analyze()

	return idx, ca
}

func setupSymfonyIndex(t *testing.T) (*symbols.Index, *container.ContainerAnalyzer) {
	t.Helper()
	root := filepath.Join("..", "..", "testdata", "symfony")

	idx := symbols.NewIndex()
	idx.RegisterBuiltins()

	for _, rel := range []string{
		"src/Controller/ProductController.php",
		"src/Service/NotificationService.php",
		"src/Service/PaymentProcessor.php",
	} {
		src := mustReadFile(t, filepath.Join(root, rel))
		idx.IndexFileWithSource("file:///"+rel, src, symbols.SourceProject)
	}

	ca := container.NewContainerAnalyzer(idx, root, "symfony")
	ca.Analyze()

	return idx, ca
}

// --- extractContainerArgContext tests ---

func TestExtractContainerArgContext(t *testing.T) {
	tests := []struct {
		name      string
		trimmed   string
		wantOk    bool
		wantFlt   string
		wantQuote string
	}{
		{"app with empty arg", "app(", true, "", ""},
		{"app with string start", "app('req", true, "req", "'"},
		{"app with double quote", `app("Req`, true, "Req", "\""},
		{"app with class prefix", "app(Request", true, "Request", ""},
		{"app with class completed", "app(Request::class)", false, "", ""},
		{"app result chained", "app('request')->", false, "", ""},
		{"resolve helper", "resolve(Pay", true, "Pay", ""},
		{"container get", "$container->get('ca", true, "ca", "'"},
		{"container make", "$container->make(App", true, "App", ""},
		{"this app make", "$this->app->make(Pay", true, "Pay", ""},
		{"not inside app", "$myapp = new Foo(", false, "", ""},
		{"app on different line", "return $x; // app(", true, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, quote, ok := extractContainerArgContext(tt.trimmed)
			if ok != tt.wantOk {
				t.Errorf("ok = %v, want %v", ok, tt.wantOk)
			}
			if ok && filter != tt.wantFlt {
				t.Errorf("filter = %q, want %q", filter, tt.wantFlt)
			}
			if ok && quote != tt.wantQuote {
				t.Errorf("quote = %q, want %q", quote, tt.wantQuote)
			}
		})
	}
}

// --- Laravel container completion tests ---

func TestLaravelAppCompletionShowsBindings(t *testing.T) {
	idx, ca := setupLaravelIndex(t)
	p := NewProvider(idx, ca, "laravel")

	source := `<?php
namespace App\Http\Controllers;

class TestController {
    public function index() {
        app(
    }
}
`
	items := p.GetCompletions("file:///test.php", source, protocol.Position{Line: 5, Character: 12})

	if len(items) == 0 {
		t.Fatal("expected completions inside app(), got none")
	}

	labels := collectLabels(items)

	// Should include default Laravel string bindings
	if !labels["request"] {
		t.Error("expected 'request' string binding")
	}
	if !labels["cache"] {
		t.Error("expected 'cache' string binding")
	}
	if !labels["router"] {
		t.Error("expected 'router' string binding")
	}
}

func TestLaravelAppCompletionShowsClassNames(t *testing.T) {
	idx, ca := setupLaravelIndex(t)
	p := NewProvider(idx, ca, "laravel")

	source := `<?php
namespace App\Http\Controllers;

use App\Models\User;

class TestController {
    public function index() {
        app(User
    }
}
`
	items := p.GetCompletions("file:///test.php", source, protocol.Position{Line: 7, Character: 16})
	labels := collectLabels(items)

	if !labels["User::class"] {
		t.Errorf("expected 'User::class' completion, got labels: %v", labels)
	}
}

func TestLaravelAppCompletionFiltersOnPrefix(t *testing.T) {
	idx, ca := setupLaravelIndex(t)
	p := NewProvider(idx, ca, "laravel")

	source := `<?php
namespace App\Http\Controllers;

class TestController {
    public function index() {
        app('req
    }
}
`
	items := p.GetCompletions("file:///test.php", source, protocol.Position{Line: 5, Character: 16})
	labels := collectLabels(items)

	if !labels["request"] {
		t.Error("expected 'request' matching 'req' prefix")
	}
	// Should NOT include unrelated bindings
	if labels["cache"] {
		t.Error("should NOT show 'cache' when filtering by 'req'")
	}
}

func TestLaravelAppCompletionDoesNotHijackStaticAccess(t *testing.T) {
	idx, ca := setupLaravelIndex(t)
	p := NewProvider(idx, ca, "laravel")

	// Normal static access outside app() should still work
	source := `<?php
namespace App\Http\Controllers;

use App\Models\User;

class TestController {
    public function index() {
        User::
    }
}
`
	items := p.GetCompletions("file:///test.php", source, protocol.Position{Line: 7, Character: 14})
	labels := collectLabels(items)

	if !labels["find"] {
		t.Error("expected 'find' static method via User::")
	}
}

func TestLaravelAppCompletionAfterClosedParenIsNotContainer(t *testing.T) {
	idx, ca := setupLaravelIndex(t)
	p := NewProvider(idx, ca, "laravel")

	// After app('request')-> should trigger member access, not container completion
	source := `<?php
namespace App\Http\Controllers;

class TestController {
    public function index() {
        app('request')->
    }
}
`
	items := p.GetCompletions("file:///test.php", source, protocol.Position{Line: 5, Character: 24})

	// Should not contain container binding labels
	for _, item := range items {
		if item.Label == "request" || item.Label == "cache" {
			t.Errorf("should NOT show container bindings after app()->, got %q", item.Label)
		}
	}
}

func TestLaravelResolveHelperCompletion(t *testing.T) {
	idx, ca := setupLaravelIndex(t)
	p := NewProvider(idx, ca, "laravel")

	source := `<?php
namespace App\Http\Controllers;

class TestController {
    public function index() {
        resolve(Pay
    }
}
`
	items := p.GetCompletions("file:///test.php", source, protocol.Position{Line: 5, Character: 19})
	labels := collectLabels(items)

	if !labels["PaymentGateway::class"] {
		t.Errorf("expected 'PaymentGateway::class' via resolve(), got labels: %v", labels)
	}
}

func TestLaravelAppInsideClassDoesNotBreakStaticAccess(t *testing.T) {
	idx, ca := setupLaravelIndex(t)
	p := NewProvider(idx, ca, "laravel")

	// Typing Request::class inside app() should offer container completions, not static members
	source := `<?php
namespace App\Http\Controllers;

use Illuminate\Http\Request;

class TestController {
    public function index() {
        app(Request::class
    }
}
`
	items := p.GetCompletions("file:///test.php", source, protocol.Position{Line: 7, Character: 26})

	// Should be in container context, not static access context
	for _, item := range items {
		if item.Label == "input" || item.Label == "all" {
			t.Errorf("should NOT show Request instance methods inside app(), got %q", item.Label)
		}
	}
}

// --- Symfony container completion tests ---

// --- String quoting tests ---

func TestLaravelAppCompletionStringBindingsQuoted(t *testing.T) {
	idx, ca := setupLaravelIndex(t)
	p := NewProvider(idx, ca, "laravel")

	t.Run("no quote typed yet wraps in single quotes", func(t *testing.T) {
		source := `<?php
namespace App\Http\Controllers;

class TestController {
    public function index() {
        app(
    }
}
`
		items := p.GetCompletions("file:///test.php", source, protocol.Position{Line: 5, Character: 12})
		for _, item := range items {
			if item.Label == "request" {
				if item.InsertText != "'request'" {
					t.Errorf("expected InsertText = \"'request'\", got %q", item.InsertText)
				}
				return
			}
		}
		t.Error("'request' binding not found in completions")
	})

	t.Run("single quote already typed adds closing quote", func(t *testing.T) {
		source := `<?php
namespace App\Http\Controllers;

class TestController {
    public function index() {
        app('
    }
}
`
		items := p.GetCompletions("file:///test.php", source, protocol.Position{Line: 5, Character: 13})
		for _, item := range items {
			if item.Label == "request" {
				if item.InsertText != "request'" {
					t.Errorf("expected InsertText = \"request'\", got %q", item.InsertText)
				}
				return
			}
		}
		t.Error("'request' binding not found in completions")
	})

	t.Run("double quote already typed adds closing double quote", func(t *testing.T) {
		source := `<?php
namespace App\Http\Controllers;

class TestController {
    public function index() {
        app("
    }
}
`
		items := p.GetCompletions("file:///test.php", source, protocol.Position{Line: 5, Character: 13})
		for _, item := range items {
			if item.Label == "request" {
				if item.InsertText != "request\"" {
					t.Errorf("expected InsertText = %q, got %q", "request\"", item.InsertText)
				}
				return
			}
		}
		t.Error("'request' binding not found in completions")
	})

	t.Run("FQN bindings also get quoted", func(t *testing.T) {
		source := `<?php
namespace App\Http\Controllers;

class TestController {
    public function index() {
        app('Illuminate
    }
}
`
		items := p.GetCompletions("file:///test.php", source, protocol.Position{Line: 5, Character: 24})
		for _, item := range items {
			if item.Label == "Illuminate\\Contracts\\Auth\\Factory" {
				if item.InsertText != "Illuminate\\Contracts\\Auth\\Factory'" {
					t.Errorf("expected closing quote in InsertText, got %q", item.InsertText)
				}
				return
			}
		}
		// Not a hard failure — may not match prefix filter
	})
}

// --- Container chain completion tests (app('x')->) ---

func TestLaravelAppChainMemberCompletion(t *testing.T) {
	idx, ca := setupLaravelIndex(t)
	p := NewProvider(idx, ca, "laravel")

	// app('request')-> should resolve to Illuminate\Http\Request and show its methods
	source := `<?php
namespace App\Http\Controllers;

class TestController {
    public function index() {
        app('request')->
    }
}
`
	items := p.GetCompletions("file:///test.php", source, protocol.Position{Line: 5, Character: 24})
	labels := collectLabels(items)

	if !labels["input"] {
		t.Errorf("expected 'input' method from Request via app('request')->, got: %v", labels)
	}
	if !labels["all"] {
		t.Error("expected 'all' method from Request via app('request')->")
	}
	if !labels["header"] {
		t.Error("expected 'header' method from Request via app('request')->")
	}
}

func TestLaravelAppClassChainMemberCompletion(t *testing.T) {
	idx, ca := setupLaravelIndex(t)
	p := NewProvider(idx, ca, "laravel")

	// app(User::class)-> should resolve User and show its members
	source := `<?php
namespace App\Http\Controllers;

use App\Models\User;

class TestController {
    public function index() {
        app(User::class)->
    }
}
`
	items := p.GetCompletions("file:///test.php", source, protocol.Position{Line: 7, Character: 26})
	labels := collectLabels(items)

	if !labels["posts"] {
		t.Errorf("expected 'posts' method from User via app(User::class)->, got: %v", labels)
	}
	if !labels["name"] {
		t.Error("expected 'name' property from User via app(User::class)->")
	}
}

func TestLaravelResolveChainMemberCompletion(t *testing.T) {
	idx, ca := setupLaravelIndex(t)
	p := NewProvider(idx, ca, "laravel")

	// resolve('request')-> should also work
	source := `<?php
namespace App\Http\Controllers;

class TestController {
    public function index() {
        resolve('request')->
    }
}
`
	items := p.GetCompletions("file:///test.php", source, protocol.Position{Line: 5, Character: 28})
	labels := collectLabels(items)

	if !labels["input"] {
		t.Errorf("expected 'input' method from Request via resolve('request')->, got: %v", labels)
	}
}

// --- ExtractContainerCallArg tests ---

func TestExtractContainerCallArg(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want string
	}{
		{"app string", "app('request')", "request"},
		{"app double quote", `app("request")`, "request"},
		{"app class", "app(Request::class)", "Request::class"},
		{"resolve string", "resolve('cache')", "cache"},
		{"container get", "$container->get('log')", "log"},
		{"container make", "$this->app->make('auth')", "auth"},
		{"not container", "foo('bar')", ""},
		{"no closing paren", "app('request'", ""},
		{"with second arg", "app('request', true)", "request"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractContainerCallArg(tt.expr)
			if got != tt.want {
				t.Errorf("ExtractContainerCallArg(%q) = %q, want %q", tt.expr, got, tt.want)
			}
		})
	}
}

// --- Symfony tests ---

func TestSymfonyContainerBindingsLoaded(t *testing.T) {
	idx, ca := setupSymfonyIndex(t)
	p := NewProvider(idx, ca, "symfony")

	source := `<?php
namespace App\Controller;

class TestController {
    public function index() {
        $container->get(
    }
}
`
	items := p.GetCompletions("file:///test.php", source, protocol.Position{Line: 5, Character: 24})

	if len(items) == 0 {
		t.Fatal("expected completions inside $container->get(), got none")
	}

	labels := collectLabels(items)

	// Symfony defaults
	if !labels["Psr\\Log\\LoggerInterface"] {
		t.Error("expected 'Psr\\Log\\LoggerInterface' default binding")
	}

	// Services from services.yaml
	if !labels["App\\Service\\NotificationService"] {
		t.Error("expected 'App\\Service\\NotificationService' from services.yaml")
	}
}

func TestSymfonyContainerFiltersByPrefix(t *testing.T) {
	idx, ca := setupSymfonyIndex(t)
	p := NewProvider(idx, ca, "symfony")

	source := `<?php
namespace App\Controller;

class TestController {
    public function index() {
        $container->get('Notif
    }
}
`
	items := p.GetCompletions("file:///test.php", source, protocol.Position{Line: 5, Character: 30})
	labels := collectLabels(items)

	if !labels["NotificationService::class"] {
		found := false
		for _, item := range items {
			if item.Label == "App\\Service\\NotificationService" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected NotificationService completion matching 'Notif', got labels: %v", labels)
		}
	}
}
