package symbols

import "testing"

func TestIndexStoresResolvedTemplateBounds(t *testing.T) {
	idx := NewIndex()
	idx.IndexFile("file:///templates.php", `<?php
namespace App;

class User {}

/**
 * @template TUser of User
 */
class Repository {}
`)

	sym := idx.Lookup("App\\Repository")
	if sym == nil {
		t.Fatal("expected App\\Repository symbol")
	}
	if len(sym.Templates) != 1 {
		t.Fatalf("expected 1 template, got %d", len(sym.Templates))
	}
	if sym.Templates[0].Name != "TUser" {
		t.Fatalf("unexpected template name %q", sym.Templates[0].Name)
	}
	if sym.Templates[0].Bound != "App\\User" {
		t.Fatalf("unexpected template bound %q", sym.Templates[0].Bound)
	}
}
