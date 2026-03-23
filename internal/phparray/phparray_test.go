package phparray

import (
	"strings"
	"testing"
)

func TestParseLiteralToShape(t *testing.T) {
	t.Run("simple key-value", func(t *testing.T) {
		fields := ParseLiteralToShape("['name' => 'John', 'age' => 30]")
		if len(fields) != 2 {
			t.Fatalf("expected 2 fields, got %d", len(fields))
		}
		if fields[0].Key != "name" || fields[0].Type != "string" {
			t.Errorf("field[0] = %+v", fields[0])
		}
		if fields[1].Key != "age" || fields[1].Type != "int" {
			t.Errorf("field[1] = %+v", fields[1])
		}
	})

	t.Run("nested arrays", func(t *testing.T) {
		fields := ParseLiteralToShape("['db' => ['host' => 'localhost']]")
		if len(fields) != 1 {
			t.Fatalf("expected 1 field, got %d", len(fields))
		}
		if fields[0].Key != "db" || !strings.HasPrefix(fields[0].Type, "array{") {
			t.Errorf("field = %+v", fields[0])
		}
	})

	t.Run("empty array", func(t *testing.T) {
		fields := ParseLiteralToShape("[]")
		if len(fields) != 0 {
			t.Errorf("expected 0 fields, got %d", len(fields))
		}
	})

	t.Run("invalid input", func(t *testing.T) {
		if ParseLiteralToShape("not an array") != nil {
			t.Error("expected nil for non-array")
		}
		if ParseLiteralToShape("") != nil {
			t.Error("expected nil for empty")
		}
	})
}

func TestParseLiteralEntries(t *testing.T) {
	t.Run("multiple entries", func(t *testing.T) {
		fields := ParseLiteralEntries("'a' => 1, 'b' => 'hello'")
		if len(fields) != 2 {
			t.Fatalf("expected 2, got %d", len(fields))
		}
	})

	t.Run("trailing comma", func(t *testing.T) {
		fields := ParseLiteralEntries("'a' => 1, ")
		if len(fields) != 1 {
			t.Fatalf("expected 1, got %d", len(fields))
		}
	})

	t.Run("no key (indexed) returns nil entry", func(t *testing.T) {
		fields := ParseLiteralEntries("'value1', 'value2'")
		if len(fields) != 0 {
			t.Errorf("expected 0 (no => arrow), got %d", len(fields))
		}
	})
}

func TestParseLiteralEntry(t *testing.T) {
	t.Run("string key and value", func(t *testing.T) {
		f := ParseLiteralEntry("'name' => 'John'")
		if f == nil || f.Key != "name" || f.Type != "string" {
			t.Errorf("got %+v", f)
		}
	})

	t.Run("int value", func(t *testing.T) {
		f := ParseLiteralEntry("'count' => 42")
		if f == nil || f.Key != "count" || f.Type != "int" {
			t.Errorf("got %+v", f)
		}
	})

	t.Run("bool value", func(t *testing.T) {
		f := ParseLiteralEntry("'active' => true")
		if f == nil || f.Key != "active" || f.Type != "bool" {
			t.Errorf("got %+v", f)
		}
	})

	t.Run("null value", func(t *testing.T) {
		f := ParseLiteralEntry("'data' => null")
		if f == nil || f.Key != "data" || f.Type != "null" {
			t.Errorf("got %+v", f)
		}
	})

	t.Run("empty entry", func(t *testing.T) {
		if ParseLiteralEntry("") != nil {
			t.Error("expected nil for empty")
		}
	})

	t.Run("no arrow returns nil", func(t *testing.T) {
		if ParseLiteralEntry("'just a value'") != nil {
			t.Error("expected nil without =>")
		}
	})

	t.Run("double quoted key", func(t *testing.T) {
		f := ParseLiteralEntry(`"key" => "val"`)
		if f == nil || f.Key != "key" {
			t.Errorf("got %+v", f)
		}
	})
}

func TestInferValueType(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"'hello'", "string"},
		{`"world"`, "string"},
		{"42", "int"},
		{"-5", "int"},
		{"3.14", "float"},
		{"1e5", "float"},
		{"true", "bool"},
		{"false", "bool"},
		{"null", "null"},
		{"['a' => 1]", "array{a: int}"},
		{"[]", "array"},
		{"$var", "mixed"},
		{"", "mixed"},
		{"SomeClass::create()", "mixed"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := InferValueType(tt.input); got != tt.want {
				t.Errorf("InferValueType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCollectArrayLiteral(t *testing.T) {
	t.Run("single line", func(t *testing.T) {
		lines := []string{"return ['a' => 1, 'b' => 2];"}
		got := CollectArrayLiteral(lines, 0)
		if got != "['a' => 1, 'b' => 2]" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("multi-line", func(t *testing.T) {
		lines := []string{"return [", "    'a' => 1,", "    'b' => 2,", "];"}
		got := CollectArrayLiteral(lines, 0)
		if !strings.HasPrefix(got, "[") || !strings.HasSuffix(got, "]") {
			t.Errorf("got %q", got)
		}
	})

	t.Run("nested arrays", func(t *testing.T) {
		lines := []string{"['outer' => ['inner' => 1]]"}
		got := CollectArrayLiteral(lines, 0)
		if got != "['outer' => ['inner' => 1]]" {
			t.Errorf("got %q", got)
		}
	})
}

func TestCollectReturnArray(t *testing.T) {
	t.Run("with line comments", func(t *testing.T) {
		lines := []string{"return [", "    // comment", "    'a' => 1,", "];"}
		got := CollectReturnArray(lines, 0)
		if !strings.Contains(got, "'a' => 1") {
			t.Errorf("should contain key, got %q", got)
		}
	})

	t.Run("with block comments", func(t *testing.T) {
		lines := []string{"return [", "    /* comment */", "    'b' => 2,", "];"}
		got := CollectReturnArray(lines, 0)
		if !strings.Contains(got, "'b' => 2") {
			t.Errorf("should contain key, got %q", got)
		}
	})
}
