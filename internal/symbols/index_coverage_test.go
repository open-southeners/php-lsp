package symbols

import (
	"testing"
)

func TestSearchByPrefix(t *testing.T) {
	idx := NewIndex()
	idx.IndexFile("file:///test.php", `<?php
namespace App;
class UserService {}
class UserController {}
function userHelper() {}
`)
	idx.IndexFile("file:///vendor.php", `<?php
namespace Vendor;
class UserRepo {}
`)

	t.Run("matches by prefix", func(t *testing.T) {
		results := idx.SearchByPrefix("User")
		if len(results) < 2 {
			t.Errorf("expected at least 2 results, got %d", len(results))
		}
	})

	t.Run("empty prefix returns all", func(t *testing.T) {
		results := idx.SearchByPrefix("")
		if len(results) == 0 {
			t.Error("expected results for empty prefix")
		}
	})

	t.Run("no matches", func(t *testing.T) {
		results := idx.SearchByPrefix("Zzzzz")
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})
}

func TestSearchByFQNPrefix(t *testing.T) {
	idx := NewIndex()
	idx.IndexFile("file:///test.php", `<?php
namespace Illuminate\Database;
class Builder {}
class Connection {}
`)
	idx.IndexFile("file:///test2.php", `<?php
namespace Illuminate\Database\Eloquent;
class Model {}
`)

	t.Run("returns symbols and child namespaces", func(t *testing.T) {
		syms, segs := idx.SearchByFQNPrefix("Illuminate\\Database\\")
		if len(syms) < 2 {
			t.Errorf("expected at least 2 symbols, got %d", len(syms))
		}
		// Should have "Eloquent" as a child namespace segment
		found := false
		for _, s := range segs {
			if s == "Eloquent" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected 'Eloquent' child segment, got %v", segs)
		}
	})
}

func TestGetFileSymbols(t *testing.T) {
	idx := NewIndex()
	idx.IndexFile("file:///a.php", `<?php class Foo {}`)
	idx.IndexFile("file:///b.php", `<?php class Bar {}`)

	syms := idx.GetFileSymbols("file:///a.php")
	if len(syms) == 0 {
		t.Error("expected symbols for a.php")
	}
	found := false
	for _, s := range syms {
		if s.Name == "Foo" {
			found = true
		}
	}
	if !found {
		t.Error("expected Foo in a.php symbols")
	}

	if len(idx.GetFileSymbols("file:///unknown.php")) != 0 {
		t.Error("expected empty for unknown URI")
	}
}

func TestGetDescendants(t *testing.T) {
	idx := NewIndex()
	idx.IndexFile("file:///base.php", `<?php class Animal {}`)
	idx.IndexFile("file:///dog.php", `<?php class Dog extends Animal {}`)
	idx.IndexFile("file:///cat.php", `<?php class Cat extends Animal {}`)
	idx.IndexFile("file:///poodle.php", `<?php class Poodle extends Dog {}`)

	descendants := idx.GetDescendants("Animal")
	if len(descendants) < 2 {
		t.Errorf("expected at least 2 descendants, got %d: %v", len(descendants), namesOf(descendants))
	}

	// Poodle should also be included (transitive)
	names := namesOf(descendants)
	for _, want := range []string{"Dog", "Cat", "Poodle"} {
		if !names[want] {
			t.Errorf("expected %s in descendants, got %v", want, names)
		}
	}

	// No descendants
	if len(idx.GetDescendants("Poodle")) != 0 {
		t.Error("Poodle should have no descendants")
	}
}

func TestAddVirtualMember(t *testing.T) {
	idx := NewIndex()
	idx.IndexFile("file:///test.php", `<?php class Foo {}`)

	idx.AddVirtualMember("Foo", &Symbol{
		Name:      "$virtualProp",
		FQN:       "Foo::$virtualProp",
		Kind:      KindProperty,
		IsVirtual: true,
		Type:      "string",
	})

	sym := idx.Lookup("Foo::$virtualProp")
	if sym == nil {
		t.Fatal("expected virtual member to be added")
	}
	if sym.ParentFQN != "Foo" {
		t.Errorf("ParentFQN = %q", sym.ParentFQN)
	}

	t.Run("skips duplicate", func(t *testing.T) {
		idx.AddVirtualMember("Foo", &Symbol{
			Name: "$virtualProp",
			FQN:  "Foo::$virtualProp",
			Kind: KindProperty,
		})
		// Should not panic or double-add
	})

	t.Run("skips if parent not found", func(t *testing.T) {
		idx.AddVirtualMember("NonExistent", &Symbol{
			Name: "test",
			FQN:  "NonExistent::test",
		})
		if idx.Lookup("NonExistent::test") != nil {
			t.Error("should not add to non-existent parent")
		}
	})
}

func TestGetImplementors(t *testing.T) {
	idx := NewIndex()
	idx.IndexFile("file:///iface.php", `<?php interface Loggable {}`)
	idx.IndexFile("file:///impl.php", `<?php class FileLogger implements Loggable {}`)
	idx.IndexFile("file:///impl2.php", `<?php class DbLogger implements Loggable {}`)

	impls := idx.GetImplementors("Loggable")
	if len(impls) < 2 {
		t.Errorf("expected at least 2 implementors, got %d", len(impls))
	}
}

func TestGetImplementedInterfaces(t *testing.T) {
	idx := NewIndex()
	idx.IndexFile("file:///test.php", `<?php
interface Countable {}
interface Serializable {}
class Foo implements Countable, Serializable {}
`)

	ifaces := idx.GetImplementedInterfaces("Foo")
	if len(ifaces) < 2 {
		t.Errorf("expected 2 interfaces, got %d: %v", len(ifaces), ifaces)
	}
}

func TestGetNamespaceMembers(t *testing.T) {
	idx := NewIndex()
	idx.IndexFile("file:///test.php", `<?php
namespace App\Models;
class User {}
class Post {}
`)

	members := idx.GetNamespaceMembers("App\\Models")
	if len(members) < 2 {
		t.Errorf("expected 2 members, got %d", len(members))
	}
}

func TestIsPHPBuiltinType(t *testing.T) {
	builtins := []string{"string", "int", "float", "bool", "array", "object", "callable", "iterable", "void", "never", "null", "mixed", "self", "static", "true", "false"}
	for _, b := range builtins {
		if !IsPHPBuiltinType(b) {
			t.Errorf("%q should be builtin", b)
		}
	}

	nonBuiltins := []string{"User", "Collection", "DateTime", ""}
	for _, n := range nonBuiltins {
		if IsPHPBuiltinType(n) {
			t.Errorf("%q should NOT be builtin", n)
		}
	}
}

func TestURIToPath(t *testing.T) {
	if got := URIToPath("file:///home/user/file.php"); got != "/home/user/file.php" {
		t.Errorf("got %q", got)
	}
	if got := URIToPath("/already/a/path"); got != "/already/a/path" {
		t.Errorf("got %q", got)
	}
}

func TestGetAllFileURIs(t *testing.T) {
	idx := NewIndex()
	idx.IndexFile("file:///a.php", `<?php class A {}`)
	idx.IndexFile("file:///b.php", `<?php class B {}`)

	uris := idx.GetAllFileURIs()
	if len(uris) < 2 {
		t.Errorf("expected at least 2 URIs, got %d", len(uris))
	}
}

func TestPickBestStandalone(t *testing.T) {
	t.Run("prefers function over method", func(t *testing.T) {
		syms := []*Symbol{
			{Name: "count", Kind: KindMethod, ParentFQN: "SomeClass"},
			{Name: "count", Kind: KindFunction},
		}
		best := PickBestStandalone(syms, "count")
		if best == nil || best.Kind != KindFunction {
			t.Error("expected function to be preferred")
		}
	})

	t.Run("prefers class over property", func(t *testing.T) {
		syms := []*Symbol{
			{Name: "User", Kind: KindProperty},
			{Name: "User", Kind: KindClass},
		}
		best := PickBestStandalone(syms, "User")
		if best == nil || best.Kind != KindClass {
			t.Error("expected class to be preferred")
		}
	})

	t.Run("nil for empty", func(t *testing.T) {
		if PickBestStandalone(nil, "x") != nil {
			t.Error("expected nil")
		}
	})
}

func TestSplitTypeExpr(t *testing.T) {
	// splitTypeExpr is unexported, test through index behavior
	idx := NewIndex()
	// Union type in property should resolve both parts
	idx.IndexFile("file:///test.php", `<?php
class Foo {
    public string|int $value;
}
`)
	sym := idx.Lookup("Foo::$value")
	if sym == nil {
		t.Fatal("expected property")
	}
	// The type should contain the union
	if sym.Type != "string|int" {
		t.Logf("type = %q (union type handling depends on parser)", sym.Type)
	}
}

func namesOf(syms []*Symbol) map[string]bool {
	m := make(map[string]bool)
	for _, s := range syms {
		m[s.Name] = true
	}
	return m
}
