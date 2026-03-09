package hover

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/open-southeners/php-lsp/internal/container"
	"github.com/open-southeners/php-lsp/internal/protocol"
	"github.com/open-southeners/php-lsp/internal/symbols"
)

func testdataPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "project")
}

func readTestFile(t *testing.T, relPath string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(testdataPath(), relPath))
	if err != nil {
		t.Fatalf("failed to read %s: %v", relPath, err)
	}
	return string(content)
}

func setupProvider(t *testing.T) (*Provider, string) {
	t.Helper()
	root := testdataPath()
	idx := symbols.NewIndex()
	idx.RegisterBuiltins()

	idx.IndexFile("file:///project/vendor/monolog/monolog/src/Monolog/Logger.php",
		readTestFile(t, "vendor/monolog/monolog/src/Monolog/Logger.php"))
	idx.IndexFile("file:///project/vendor/monolog/monolog/src/Monolog/Handler/StreamHandler.php",
		readTestFile(t, "vendor/monolog/monolog/src/Monolog/Handler/StreamHandler.php"))
	idx.IndexFile("file:///project/src/Service.php",
		readTestFile(t, "src/Service.php"))

	ca := container.NewContainerAnalyzer(idx, root, "none")
	provider := NewProvider(idx, ca, "none")

	source := readTestFile(t, "src/Service.php")
	return provider, source
}

// charPosOf returns the line and character position of the last occurrence
// of target on the line containing lineHint. Using LastIndex avoids matching
// substrings inside variable names (e.g. "handle" inside "$handler").
func charPosOf(t *testing.T, source, target, lineHint string) protocol.Position {
	t.Helper()
	for i, line := range strings.Split(source, "\n") {
		if lineHint != "" && !strings.Contains(line, lineHint) {
			continue
		}
		col := strings.LastIndex(line, target)
		if col >= 0 {
			return protocol.Position{Line: i, Character: col}
		}
	}
	t.Fatalf("could not find %q (hint: %q) in source", target, lineHint)
	return protocol.Position{}
}

func TestHoverInstanceMethodViaProperty(t *testing.T) {
	p, src := setupProvider(t)
	// $this->logger->info('hello') — hover on "info"
	pos := charPosOf(t, src, "info", "$this->logger->info")
	hover := p.GetHover("file:///project/src/Service.php", src, pos)
	if hover == nil {
		t.Fatal("expected hover result")
	}
	val := hover.Contents.Value
	if !strings.Contains(val, "function info") {
		t.Errorf("expected method signature, got:\n%s", val)
	}
	if !strings.Contains(val, "Monolog\\Logger") {
		t.Errorf("expected parent class FQN, got:\n%s", val)
	}
}

func TestHoverMethodChainThroughReturnType(t *testing.T) {
	p, src := setupProvider(t)
	// $handler->getLogger()->info('via handler') — hover on second "info"
	pos := charPosOf(t, src, "info", "getLogger()->info")
	hover := p.GetHover("file:///project/src/Service.php", src, pos)
	if hover == nil {
		t.Fatal("expected hover result")
	}
	val := hover.Contents.Value
	if !strings.Contains(val, "function info") {
		t.Errorf("expected method signature, got:\n%s", val)
	}
	if !strings.Contains(val, "Monolog\\Logger") {
		t.Errorf("expected parent class FQN, got:\n%s", val)
	}
}

func TestHoverVendorMethod(t *testing.T) {
	p, src := setupProvider(t)
	// $handler->handle(['message' => 'test']) — hover on "handle"
	pos := charPosOf(t, src, "handle", "$handler->handle")
	hover := p.GetHover("file:///project/src/Service.php", src, pos)
	if hover == nil {
		t.Fatal("expected hover result")
	}
	val := hover.Contents.Value
	if !strings.Contains(val, "function handle") {
		t.Errorf("expected method signature, got:\n%s", val)
	}
	if !strings.Contains(val, "Monolog\\Handler\\StreamHandler") {
		t.Errorf("expected parent class FQN, got:\n%s", val)
	}
}

func TestHoverStaticMethod(t *testing.T) {
	p, src := setupProvider(t)
	// Logger::create('app') — hover on "create"
	pos := charPosOf(t, src, "create", "Logger::create")
	hover := p.GetHover("file:///project/src/Service.php", src, pos)
	if hover == nil {
		t.Fatal("expected hover result")
	}
	val := hover.Contents.Value
	if !strings.Contains(val, "static") {
		t.Errorf("expected static keyword, got:\n%s", val)
	}
	if !strings.Contains(val, "function create") {
		t.Errorf("expected method signature, got:\n%s", val)
	}
}

func TestHoverSelfMethod(t *testing.T) {
	p, src := setupProvider(t)
	// self::helper() — hover on "helper"
	pos := charPosOf(t, src, "helper", "self::helper")
	hover := p.GetHover("file:///project/src/Service.php", src, pos)
	if hover == nil {
		t.Fatal("expected hover result")
	}
	val := hover.Contents.Value
	if !strings.Contains(val, "function helper") {
		t.Errorf("expected method signature, got:\n%s", val)
	}
	if !strings.Contains(val, "App\\Service") {
		t.Errorf("expected parent class FQN, got:\n%s", val)
	}
}

func TestHoverPropertyAccess(t *testing.T) {
	p, src := setupProvider(t)
	// $this->logger in assignment context — hover on "logger"
	pos := charPosOf(t, src, "logger", "$this->logger = $logger")
	hover := p.GetHover("file:///project/src/Service.php", src, pos)
	if hover == nil {
		t.Fatal("expected hover result")
	}
	val := hover.Contents.Value
	if !strings.Contains(val, "$logger") {
		t.Errorf("expected property name, got:\n%s", val)
	}
	if !strings.Contains(val, "Logger") {
		t.Errorf("expected Logger type, got:\n%s", val)
	}
}

func TestHoverVariableWithTypeHint(t *testing.T) {
	p, src := setupProvider(t)
	// public function __construct(Logger $logger) — hover on "$logger"
	pos := charPosOf(t, src, "$logger", "__construct(Logger $logger")
	hover := p.GetHover("file:///project/src/Service.php", src, pos)
	if hover == nil {
		t.Fatal("expected hover result")
	}
	val := hover.Contents.Value
	if !strings.Contains(val, "Monolog\\Logger") {
		t.Errorf("expected FQN type, got:\n%s", val)
	}
}

func TestHoverClassInUseStatement(t *testing.T) {
	p, src := setupProvider(t)
	// use Monolog\Logger — hover on "Logger" part (which is the whole FQN via getWordAt)
	pos := charPosOf(t, src, "Logger", "use Monolog\\Logger")
	hover := p.GetHover("file:///project/src/Service.php", src, pos)
	if hover == nil {
		t.Fatal("expected hover result")
	}
	val := hover.Contents.Value
	if !strings.Contains(val, "class Monolog\\Logger") {
		t.Errorf("expected class info, got:\n%s", val)
	}
}

func TestHoverClassNameInTypeDecl(t *testing.T) {
	p, src := setupProvider(t)
	// private Logger $logger — hover on "Logger"
	pos := charPosOf(t, src, "Logger", "private Logger $logger")
	hover := p.GetHover("file:///project/src/Service.php", src, pos)
	if hover == nil {
		t.Fatal("expected hover result")
	}
	val := hover.Contents.Value
	if !strings.Contains(val, "class Monolog\\Logger") {
		t.Errorf("expected class info, got:\n%s", val)
	}
}

func TestHoverDocBlockTags(t *testing.T) {
	p, src := setupProvider(t)
	// $handler->handle() has @param and @return in its docblock
	pos := charPosOf(t, src, "handle", "$handler->handle")
	hover := p.GetHover("file:///project/src/Service.php", src, pos)
	if hover == nil {
		t.Fatal("expected hover result")
	}
	val := hover.Contents.Value
	if !strings.Contains(val, "Parameters:") {
		t.Errorf("expected @param info, got:\n%s", val)
	}
	if !strings.Contains(val, "Returns:") {
		t.Errorf("expected @return info, got:\n%s", val)
	}
}

func TestHoverNewExpression(t *testing.T) {
	p, src := setupProvider(t)
	// $handler = new StreamHandler() — hover on "StreamHandler"
	pos := charPosOf(t, src, "StreamHandler", "new StreamHandler")
	hover := p.GetHover("file:///project/src/Service.php", src, pos)
	if hover == nil {
		t.Fatal("expected hover result")
	}
	val := hover.Contents.Value
	if !strings.Contains(val, "class Monolog\\Handler\\StreamHandler") {
		t.Errorf("expected class info, got:\n%s", val)
	}
}

func TestHoverNoResult(t *testing.T) {
	p, src := setupProvider(t)
	// Hover on a keyword that isn't a symbol
	pos := charPosOf(t, src, "void", "public function run")
	hover := p.GetHover("file:///project/src/Service.php", src, pos)
	// void is not indexed, should return nil
	if hover != nil {
		t.Errorf("expected nil hover for keyword, got: %s", hover.Contents.Value)
	}
}

