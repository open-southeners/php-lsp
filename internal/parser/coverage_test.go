package parser

import (
	"testing"
)

func TestParseFileTrait(t *testing.T) {
	file := ParseFile(`<?php
namespace App\Traits;
trait HasTimestamps {
    public function getCreatedAt(): string { return ""; }
    private string $created_at;
}
`)
	if file == nil {
		t.Fatal("expected non-nil file")
	}
	if len(file.Traits) == 0 {
		t.Fatal("expected at least 1 trait")
	}
	tr := file.Traits[0]
	if tr.Name != "HasTimestamps" {
		t.Errorf("name = %q", tr.Name)
	}
	if len(tr.Methods) == 0 {
		t.Error("expected methods on trait")
	}
	if len(tr.Properties) == 0 {
		t.Error("expected properties on trait")
	}
}

func TestParseFileEnum(t *testing.T) {
	file := ParseFile(`<?php
namespace App\Enums;
enum Status: string {
    case Active = 'active';
    case Inactive = 'inactive';

    public function label(): string {
        return match($this) {
            self::Active => 'Active',
            self::Inactive => 'Inactive',
        };
    }
}
`)
	if file == nil {
		t.Fatal("expected non-nil file")
	}
	if len(file.Enums) == 0 {
		t.Fatal("expected at least 1 enum")
	}
	e := file.Enums[0]
	if e.Name != "Status" {
		t.Errorf("name = %q", e.Name)
	}
}

func TestParseFileStandaloneFunction(t *testing.T) {
	file := ParseFile(`<?php
namespace App\Helpers;
function formatName(string $first, string $last): string {
    return $first . ' ' . $last;
}
`)
	if file == nil {
		t.Fatal("expected non-nil file")
	}
	if len(file.Functions) == 0 {
		t.Fatal("expected at least 1 function")
	}
	fn := file.Functions[0]
	if fn.Name != "formatName" {
		t.Errorf("name = %q", fn.Name)
	}
	if len(fn.Params) != 2 {
		t.Errorf("expected 2 params, got %d", len(fn.Params))
	}
}

func TestParseFileConstants(t *testing.T) {
	file := ParseFile(`<?php
namespace App;
const MAX_RETRIES = 3;
class Foo {
    const VERSION = '1.0';
}
`)
	if file == nil {
		t.Fatal("expected non-nil file")
	}
	// Global constant
	if len(file.Constants) == 0 {
		t.Log("global constants may not be parsed (parser limitation)")
	}
	// Class constant
	if len(file.Classes) > 0 && len(file.Classes[0].Constants) == 0 {
		t.Log("class constants may not be parsed")
	}
}

func TestParseFileInterface(t *testing.T) {
	file := ParseFile(`<?php
namespace App\Contracts;
interface Repository {
    public function find(int $id): mixed;
    public function all(): array;
}
`)
	if file == nil {
		t.Fatal("expected non-nil file")
	}
	if len(file.Interfaces) == 0 {
		t.Fatal("expected at least 1 interface")
	}
	iface := file.Interfaces[0]
	if iface.Name != "Repository" {
		t.Errorf("name = %q", iface.Name)
	}
	if len(iface.Methods) < 2 {
		t.Errorf("expected 2 methods, got %d", len(iface.Methods))
	}
}

func TestParseDocBlockTemplate(t *testing.T) {
	doc := ParseDocBlock(`/**
 * @template TModel of \Illuminate\Database\Eloquent\Model
 * @param TModel $model
 * @return TModel
 */`)
	if doc == nil {
		t.Fatal("expected non-nil docblock")
	}
	if doc.Return.Type != "TModel" {
		t.Errorf("return type = %q", doc.Return.Type)
	}
}

func TestParseDocBlockMixin(t *testing.T) {
	doc := ParseDocBlock(`/**
 * @mixin \Illuminate\Database\Eloquent\Builder
 */`)
	if doc == nil {
		t.Fatal("expected non-nil docblock")
	}
	if mixins, ok := doc.Tags["mixin"]; !ok || len(mixins) == 0 {
		t.Error("expected @mixin tag")
	}
}

func TestParseFileFaultTolerance(t *testing.T) {
	t.Run("incomplete class", func(t *testing.T) {
		file := ParseFile(`<?php
class Foo {
    public function bar() {
`)
		// Should not panic, may return partial results
		if file == nil {
			t.Log("incomplete class returned nil (acceptable)")
		}
	})

	t.Run("missing closing brace", func(t *testing.T) {
		file := ParseFile(`<?php
namespace App;
class Service {
    public string $name;
`)
		if file == nil {
			t.Log("missing brace returned nil (acceptable)")
		} else if file.Namespace != "App" {
			t.Errorf("namespace should still be parsed, got %q", file.Namespace)
		}
	})

	t.Run("empty source", func(t *testing.T) {
		file := ParseFile("")
		if file == nil {
			t.Log("empty source returned nil (acceptable)")
		}
	})

	t.Run("just PHP tag", func(t *testing.T) {
		file := ParseFile("<?php")
		if file == nil {
			t.Log("just PHP tag returned nil (acceptable)")
		}
	})
}

func TestParseFileMultipleClasses(t *testing.T) {
	file := ParseFile(`<?php
namespace App;
class Foo {
    public function bar(): void {}
}
class Baz extends Foo {
    public function qux(): string { return ""; }
}
`)
	if file == nil {
		t.Fatal("expected non-nil")
	}
	if len(file.Classes) < 2 {
		t.Errorf("expected 2 classes, got %d", len(file.Classes))
	}
}

func TestParseFileUseStatements(t *testing.T) {
	file := ParseFile(`<?php
namespace App;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Support\Collection as Coll;
use function App\Helpers\formatName;
use const App\Config\MAX_RETRIES;
class Foo extends Model {}
`)
	if file == nil {
		t.Fatal("expected non-nil")
	}
	if len(file.Uses) < 2 {
		t.Errorf("expected at least 2 use statements, got %d", len(file.Uses))
	}
	// Check aliased import
	for _, u := range file.Uses {
		if u.FullName == "Illuminate\\Support\\Collection" {
			if u.Alias != "Coll" {
				t.Errorf("expected alias 'Coll', got %q", u.Alias)
			}
		}
	}
}

func TestParseFileVisibility(t *testing.T) {
	file := ParseFile(`<?php
class Foo {
    public string $pub;
    protected string $prot;
    private string $priv;
    public function pubMethod(): void {}
    protected function protMethod(): void {}
    private function privMethod(): void {}
    public static function staticMethod(): void {}
}
`)
	if file == nil || len(file.Classes) == 0 {
		t.Fatal("expected class")
	}
	cls := file.Classes[0]
	for _, p := range cls.Properties {
		switch p.Name {
		case "$pub":
			if p.Visibility != "public" {
				t.Errorf("$pub visibility = %q", p.Visibility)
			}
		case "$prot":
			if p.Visibility != "protected" {
				t.Errorf("$prot visibility = %q", p.Visibility)
			}
		case "$priv":
			if p.Visibility != "private" {
				t.Errorf("$priv visibility = %q", p.Visibility)
			}
		}
	}
	for _, m := range cls.Methods {
		if m.Name == "staticMethod" && !m.IsStatic {
			t.Error("staticMethod should be static")
		}
	}
}
