package composer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// AutoloadEntry represents a single PSR-4 namespace-to-directory mapping.
type AutoloadEntry struct {
	Namespace string
	Path      string // Absolute path to the directory
	IsVendor  bool
}

type composerJSON struct {
	Autoload    autoloadBlock         `json:"autoload"`
	AutoloadDev autoloadBlock         `json:"autoload-dev"`
}

type autoloadBlock struct {
	PSR4 map[string]interface{} `json:"psr-4"`
}

type installedJSON struct {
	Packages []installedPackage `json:"packages"`
}

type installedPackage struct {
	Name        string        `json:"name"`
	InstallPath string        `json:"install-path"`
	Autoload    autoloadBlock `json:"autoload"`
}

// GetAutoloadPaths returns all PSR-4 namespace-to-directory mappings from the
// project's own composer.json (autoload + autoload-dev) and from every
// installed vendor package listed in vendor/composer/installed.json.
func GetAutoloadPaths(rootPath string) []AutoloadEntry {
	var entries []AutoloadEntry

	// 1. Project's own autoload mappings
	projectComposer := filepath.Join(rootPath, "composer.json")
	if data, err := os.ReadFile(projectComposer); err == nil {
		var cj composerJSON
		if json.Unmarshal(data, &cj) == nil {
			entries = append(entries, parsePSR4(cj.Autoload.PSR4, rootPath, false)...)
			entries = append(entries, parsePSR4(cj.AutoloadDev.PSR4, rootPath, false)...)
		}
	}

	// 2. Vendor package autoload mappings from installed.json
	composerDir := filepath.Join(rootPath, "vendor", "composer")
	installedPath := filepath.Join(composerDir, "installed.json")
	data, err := os.ReadFile(installedPath)
	if err != nil {
		return entries
	}

	// Composer v2 format: {"packages": [...]}
	// Composer v1 format: [...]
	var packages []installedPackage
	var v2 installedJSON
	if json.Unmarshal(data, &v2) == nil && v2.Packages != nil {
		packages = v2.Packages
	} else {
		json.Unmarshal(data, &packages)
	}

	for _, pkg := range packages {
		if len(pkg.Autoload.PSR4) == 0 {
			continue
		}
		// install-path is relative to vendor/composer/
		pkgDir := filepath.Join(composerDir, pkg.InstallPath)
		pkgDir = filepath.Clean(pkgDir)
		entries = append(entries, parsePSR4(pkg.Autoload.PSR4, pkgDir, true)...)
	}

	return entries
}

func parsePSR4(psr4 map[string]interface{}, basePath string, isVendor bool) []AutoloadEntry {
	var entries []AutoloadEntry
	for ns, paths := range psr4 {
		ns = strings.TrimRight(ns, "\\")
		var dirs []string
		switch v := paths.(type) {
		case string:
			dirs = []string{v}
		case []interface{}:
			for _, p := range v {
				if s, ok := p.(string); ok {
					dirs = append(dirs, s)
				}
			}
		}
		for _, dir := range dirs {
			absDir := filepath.Join(basePath, dir)
			entries = append(entries, AutoloadEntry{
				Namespace: ns,
				Path:      absDir,
				IsVendor:  isVendor,
			})
		}
	}
	return entries
}
