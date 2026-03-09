package composer

import (
	"path/filepath"
	"runtime"
	"testing"
)

func testdataPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "project")
}

func TestGetAutoloadPaths(t *testing.T) {
	root := testdataPath()
	entries := GetAutoloadPaths(root)

	if len(entries) == 0 {
		t.Fatal("expected autoload entries, got none")
	}

	var appEntry, testsEntry, vendorEntry *AutoloadEntry
	for i := range entries {
		switch entries[i].Namespace {
		case "App":
			appEntry = &entries[i]
		case "Tests":
			testsEntry = &entries[i]
		case "Monolog":
			vendorEntry = &entries[i]
		}
	}

	t.Run("project autoload", func(t *testing.T) {
		if appEntry == nil {
			t.Fatal("missing App\\ autoload entry")
		}
		if appEntry.IsVendor {
			t.Error("App\\ should not be marked as vendor")
		}
		if !filepath.IsAbs(appEntry.Path) {
			t.Errorf("expected absolute path, got %q", appEntry.Path)
		}
	})

	t.Run("project autoload-dev", func(t *testing.T) {
		if testsEntry == nil {
			t.Fatal("missing Tests\\ autoload-dev entry")
		}
		if testsEntry.IsVendor {
			t.Error("Tests\\ should not be marked as vendor")
		}
	})

	t.Run("vendor autoload", func(t *testing.T) {
		if vendorEntry == nil {
			t.Fatal("missing Monolog\\ vendor autoload entry")
		}
		if !vendorEntry.IsVendor {
			t.Error("Monolog\\ should be marked as vendor")
		}
		if !filepath.IsAbs(vendorEntry.Path) {
			t.Errorf("expected absolute path, got %q", vendorEntry.Path)
		}
	})
}

func TestGetAutoloadPathsMissing(t *testing.T) {
	entries := GetAutoloadPaths("/nonexistent/path")
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for missing path, got %d", len(entries))
	}
}

func TestGetAutoloadPathsNoVendor(t *testing.T) {
	// testdata/project has a composer.json but if we point to a subdir without one
	root := testdataPath()
	entries := GetAutoloadPaths(filepath.Join(root, "src"))
	// Should return 0 since src/ has no composer.json
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for dir without composer.json, got %d", len(entries))
	}
}
