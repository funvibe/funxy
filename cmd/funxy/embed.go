package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EmbedSpec describes a parsed --embed argument with optional @alias@ syntax.
type EmbedSpec struct {
	PhysicalPath string // directory or file path on disk (left of first @)
	Alias        string // key prefix for the script (between @@); defaults to PhysicalPath
	GlobFilter   string // optional filename filter within directory (after second @)
	HasAlias     bool   // whether @alias@ was explicitly specified
}

// parseEmbedArg parses a single --embed argument, extracting @alias@ if present.
//
// Format:
//
//	"template"                     → {PhysicalPath:"template", Alias:"template"}
//	"template/@.@"                 → {PhysicalPath:"template/", Alias:".", GlobFilter:""}
//	"template/@.@*.html"           → {PhysicalPath:"template/", Alias:".", GlobFilter:"*.html"}
//	"template@../views/template@"  → {PhysicalPath:"template", Alias:"../views/template"}
//	"template@/abs/path@"          → {PhysicalPath:"template", Alias:"/abs/path"}
func parseEmbedArg(arg string) EmbedSpec {
	firstAt := strings.Index(arg, "@")
	if firstAt == -1 {
		return EmbedSpec{PhysicalPath: arg, Alias: arg}
	}
	rest := arg[firstAt+1:]
	secondAt := strings.Index(rest, "@")
	if secondAt == -1 {
		// Only one @, treat entire string as literal path
		return EmbedSpec{PhysicalPath: arg, Alias: arg}
	}

	physical := arg[:firstAt]
	alias := rest[:secondAt]
	glob := rest[secondAt+1:]

	return EmbedSpec{
		PhysicalPath: physical,
		Alias:        alias,
		GlobFilter:   glob,
		HasAlias:     true,
	}
}

// computeEmbedKey computes the embed resource map key for a file within a directory.
//
// alias is the key prefix (what the script sees), physicalBase is the walked directory,
// filePath is the actual file on disk.
//
// The key is: join(alias, rel(physicalBase, filePath)), normalized to forward slashes.
func computeEmbedKey(alias string, physicalBase string, filePath string) string {
	relPath, err := filepath.Rel(physicalBase, filePath)
	if err != nil {
		relPath = filepath.Base(filePath)
	}
	key := filepath.Join(alias, relPath)
	return filepath.ToSlash(key)
}

// cleanPhysicalPath trims trailing slashes and normalizes empty to ".".
func cleanPhysicalPath(p string) string {
	p = strings.TrimRight(p, "/\\")
	if p == "" {
		return "."
	}
	return p
}

// collectEmbedDir walks a directory and adds files to the resources map.
// alias is the key prefix; globFilter (if non-empty) filters files by basename pattern.
func collectEmbedDir(dir string, alias string, globFilter string, resources map[string][]byte) error {
	cleanDir := cleanPhysicalPath(dir)

	info, err := os.Stat(cleanDir)
	if err != nil {
		return fmt.Errorf("cannot stat %s: %w", cleanDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory (alias is only supported for directories)", cleanDir)
	}

	// Pre-expand brace patterns in glob filter
	var globPatterns []string
	if globFilter != "" {
		globPatterns = expandBraces(globFilter)
	}

	return filepath.Walk(cleanDir, func(filePath string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}

		// Apply glob filter if specified
		if len(globPatterns) > 0 {
			baseName := filepath.Base(filePath)
			matched := false
			for _, pattern := range globPatterns {
				if ok, _ := filepath.Match(pattern, baseName); ok {
					matched = true
					break
				}
			}
			if !matched {
				return nil
			}
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("cannot read %s: %w", filePath, err)
		}

		key := computeEmbedKey(alias, cleanDir, filePath)
		resources[key] = data
		return nil
	})
}

// collectEmbedFile reads a single file and adds it to resources with the given key.
func collectEmbedFile(filePath string, key string, resources map[string][]byte) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", filePath, err)
	}
	resources[key] = data
	return nil
}
