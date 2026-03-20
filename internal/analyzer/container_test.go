package analyzer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-southeners/php-lsp/internal/container"
	"github.com/open-southeners/php-lsp/internal/protocol"
	"github.com/open-southeners/php-lsp/internal/symbols"
)

func laravelTestdataPath() string {
	return filepath.Join("..", "..", "testdata", "laravel")
}

func readLaravelFile(t *testing.T, relPath string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(laravelTestdataPath(), relPath))
	if err != nil {
		t.Fatalf("failed to read %s: %v", relPath, err)
	}
	return string(content)
}

func setupLaravelAnalyzer(t *testing.T) *Analyzer {
	t.Helper()
	root := laravelTestdataPath()
	idx := symbols.NewIndex()
	idx.RegisterBuiltins()

	for _, rel := range []string{
		"app/Models/User.php",
		"app/Models/Category.php",
		"app/Services/PaymentGateway.php",
		"app/Services/StripeGateway.php",
		"app/Services/CustomMailer.php",
		"app/Providers/AppServiceProvider.php",
		"vendor/illuminate/http/src/Request.php",
	} {
		src := readLaravelFile(t, rel)
		source := symbols.SourceProject
		if strings.HasPrefix(rel, "vendor/") {
			source = symbols.SourceVendor
		}
		idx.IndexFileWithSource("file:///"+rel, src, source)
	}

	ca := container.NewContainerAnalyzer(idx, root, "laravel")
	ca.Analyze()

	return NewAnalyzer(idx, ca)
}

func TestDefinitionAppStringBinding(t *testing.T) {
	a := setupLaravelAnalyzer(t)

	// Clicking on 'request' inside app('request') should go to Illuminate\Http\Request
	source := `<?php
namespace App\Http\Controllers;

class TestController {
    public function index() {
        app('request');
    }
}
`
	pos := protocol.Position{Line: 5, Character: 14} // on 'request'
	loc := a.FindDefinition("file:///test.php", source, pos)
	if loc == nil {
		t.Fatal("expected definition for app('request')")
	}
	if !strings.Contains(loc.URI, "Request.php") {
		t.Errorf("expected URI containing Request.php, got %s", loc.URI)
	}
}

func TestDefinitionAppClassBinding(t *testing.T) {
	a := setupLaravelAnalyzer(t)

	// app(User::class) should go to User class
	source := `<?php
namespace App\Http\Controllers;

use App\Models\User;

class TestController {
    public function index() {
        app(User::class);
    }
}
`
	pos := protocol.Position{Line: 7, Character: 13} // on 'User'
	loc := a.FindDefinition("file:///test.php", source, pos)
	if loc == nil {
		t.Fatal("expected definition for app(User::class)")
	}
	if !strings.Contains(loc.URI, "User.php") {
		t.Errorf("expected URI containing User.php, got %s", loc.URI)
	}
}

func TestDefinitionAppChainedMethod(t *testing.T) {
	a := setupLaravelAnalyzer(t)

	// app('request')->input() — clicking on 'input' should go to Request::input
	source := `<?php
namespace App\Http\Controllers;

class TestController {
    public function index() {
        app('request')->input('key');
    }
}
`
	pos := protocol.Position{Line: 5, Character: 28} // on 'input'
	loc := a.FindDefinition("file:///test.php", source, pos)
	if loc == nil {
		t.Fatal("expected definition for input() on app('request')->input()")
	}
	if !strings.Contains(loc.URI, "Request.php") {
		t.Errorf("expected URI containing Request.php, got %s", loc.URI)
	}
}

func TestDefinitionResolveHelper(t *testing.T) {
	a := setupLaravelAnalyzer(t)

	// resolve('request') should go to Request class
	source := `<?php
namespace App\Http\Controllers;

class TestController {
    public function index() {
        resolve('request');
    }
}
`
	pos := protocol.Position{Line: 5, Character: 17} // on 'request'
	loc := a.FindDefinition("file:///test.php", source, pos)
	if loc == nil {
		t.Fatal("expected definition for resolve('request')")
	}
	if !strings.Contains(loc.URI, "Request.php") {
		t.Errorf("expected URI containing Request.php, got %s", loc.URI)
	}
}
