package cli

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestParseEmbedArg(t *testing.T) {
	tests := []struct {
		input     string
		wantPhys  string
		wantAlias string
		wantGlob  string
		wantHas   bool
	}{
		// No alias — key prefix = argument itself
		{"template", "template", "template", "", false},
		{"examples/playground", "examples/playground", "examples/playground", "", false},
		{"config.json", "config.json", "config.json", "", false},
		{".", ".", ".", "", false},

		// With alias, no glob filter
		{"template@.@", "template", ".", "", true},
		{"template@../views/template@", "template", "../views/template", "", true},
		{"template@/abs/path@", "template", "/abs/path", "", true},
		{"examples/playground@.@", "examples/playground", ".", "", true},

		// With alias and glob filter
		{"template/@.@*.html", "template/", ".", "*.html", true},
		{"template/@.@*.{js,html}", "template/", ".", "*.{js,html}", true},
		{"assets/@static@*.css", "assets/", "static", "*.css", true},

		// Edge: single @ is literal (no second @)
		{"path@with_at", "path@with_at", "path@with_at", "", false},

		// Edge: empty alias (strip prefix entirely)
		{"template/@.@", "template/", ".", "", true},

		// Edge: alias with slashes
		{"src@../../lib/src@", "src", "../../lib/src", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			spec := parseEmbedArg(tt.input)
			if spec.PhysicalPath != tt.wantPhys {
				t.Errorf("PhysicalPath: got %q, want %q", spec.PhysicalPath, tt.wantPhys)
			}
			if spec.Alias != tt.wantAlias {
				t.Errorf("Alias: got %q, want %q", spec.Alias, tt.wantAlias)
			}
			if spec.GlobFilter != tt.wantGlob {
				t.Errorf("GlobFilter: got %q, want %q", spec.GlobFilter, tt.wantGlob)
			}
			if spec.HasAlias != tt.wantHas {
				t.Errorf("HasAlias: got %v, want %v", spec.HasAlias, tt.wantHas)
			}
		})
	}
}

func TestComputeEmbedKey(t *testing.T) {
	tests := []struct {
		name    string
		alias   string
		base    string
		file    string
		wantKey string
	}{
		// Default: alias = dir name, key preserves it
		{"dir default", "template", "template", filepath.Join("template", "foo.html"), "template/foo.html"},
		{"dir nested", "template", "template", filepath.Join("template", "sub", "page.html"), "template/sub/page.html"},
		{"multi-level", "examples/playground", "examples/playground",
			filepath.Join("examples", "playground", "app.js"), "examples/playground/app.js"},

		// Alias = "." strips prefix
		{"alias dot", ".", "template", filepath.Join("template", "foo.html"), "foo.html"},
		{"alias dot nested", ".", "template", filepath.Join("template", "sub", "page.html"), "sub/page.html"},
		{"alias dot multi", ".", "examples/playground",
			filepath.Join("examples", "playground", "app.js"), "app.js"},

		// Alias = relative path
		{"alias relative", "../views/template", "template",
			filepath.Join("template", "foo.html"), "../views/template/foo.html"},

		// Alias = absolute path
		{"alias absolute", "/usr/share/tpl", "template",
			filepath.Join("template", "foo.html"), "/usr/share/tpl/foo.html"},

		// Alias with .. is normalized by filepath.Join (Clean inside)
		{"alias dotdot normalized", "dir/..", "template",
			filepath.Join("template", "foo.html"), "foo.html"},
		{"alias dotdot sub", "dir/../sub", "template",
			filepath.Join("template", "foo.html"), "sub/foo.html"},
		{"alias dot normalized", ".", "template",
			filepath.Join("template", "foo.html"), "foo.html"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeEmbedKey(tt.alias, tt.base, tt.file)
			if got != tt.wantKey {
				t.Errorf("computeEmbedKey(%q, %q, %q) = %q, want %q",
					tt.alias, tt.base, tt.file, got, tt.wantKey)
			}
		})
	}
}

func TestCollectEmbedDir(t *testing.T) {
	// Create temp directory structure:
	//   template/
	//     index.html
	//     app.js
	//     sub/
	//       page.html
	tmpDir := t.TempDir()
	tplDir := filepath.Join(tmpDir, "template")
	os.MkdirAll(filepath.Join(tplDir, "sub"), 0755)
	os.WriteFile(filepath.Join(tplDir, "index.html"), []byte("<h1>Hi</h1>"), 0644)
	os.WriteFile(filepath.Join(tplDir, "app.js"), []byte("console.log('hi')"), 0644)
	os.WriteFile(filepath.Join(tplDir, "sub", "page.html"), []byte("<p>Page</p>"), 0644)

	t.Run("no alias — alias equals dir name", func(t *testing.T) {
		resources := make(map[string][]byte)
		err := collectEmbedDir(tplDir, tplDir, "", resources)
		if err != nil {
			t.Fatal(err)
		}
		// Keys should be just basenames relative to tplDir, prefixed with tplDir alias
		// But since we pass tplDir as both dir and alias, key = join(tplDir, rel) which is abs.
		// In real usage, alias = "template" and dir = "template" (relative).
		// For this test, just check the files are there.
		if len(resources) != 3 {
			t.Errorf("expected 3 resources, got %d: %v", len(resources), mapKeys(resources))
		}
	})

	t.Run("realistic: alias = dir basename", func(t *testing.T) {
		// Simulate: --embed template (from inside tmpDir)
		resources := make(map[string][]byte)
		// Use relative-like approach: alias = "template", dir = absolute tplDir
		err := collectEmbedDir(tplDir, "template", "", resources)
		if err != nil {
			t.Fatal(err)
		}
		expectKeys(t, resources, []string{
			"template/index.html",
			"template/app.js",
			"template/sub/page.html",
		})
	})

	t.Run("alias dot — strip prefix", func(t *testing.T) {
		resources := make(map[string][]byte)
		err := collectEmbedDir(tplDir, ".", "", resources)
		if err != nil {
			t.Fatal(err)
		}
		expectKeys(t, resources, []string{
			"index.html",
			"app.js",
			"sub/page.html",
		})
	})

	t.Run("alias relative", func(t *testing.T) {
		resources := make(map[string][]byte)
		err := collectEmbedDir(tplDir, "../views/template", "", resources)
		if err != nil {
			t.Fatal(err)
		}
		expectKeys(t, resources, []string{
			"../views/template/index.html",
			"../views/template/app.js",
			"../views/template/sub/page.html",
		})
	})

	t.Run("alias absolute", func(t *testing.T) {
		resources := make(map[string][]byte)
		err := collectEmbedDir(tplDir, "/usr/share/tpl", "", resources)
		if err != nil {
			t.Fatal(err)
		}
		expectKeys(t, resources, []string{
			"/usr/share/tpl/index.html",
			"/usr/share/tpl/app.js",
			"/usr/share/tpl/sub/page.html",
		})
	})

	t.Run("glob filter html only", func(t *testing.T) {
		resources := make(map[string][]byte)
		err := collectEmbedDir(tplDir, ".", "*.html", resources)
		if err != nil {
			t.Fatal(err)
		}
		expectKeys(t, resources, []string{
			"index.html",
			"sub/page.html",
		})
		if _, ok := resources["app.js"]; ok {
			t.Error("app.js should be filtered out by *.html")
		}
	})

	t.Run("glob filter js only", func(t *testing.T) {
		resources := make(map[string][]byte)
		err := collectEmbedDir(tplDir, "static", "*.js", resources)
		if err != nil {
			t.Fatal(err)
		}
		expectKeys(t, resources, []string{
			"static/app.js",
		})
	})

	t.Run("not a directory", func(t *testing.T) {
		resources := make(map[string][]byte)
		err := collectEmbedDir(filepath.Join(tplDir, "index.html"), ".", "", resources)
		if err == nil {
			t.Error("expected error for file passed as dir")
		}
	})
}

func TestCollectEmbedFile(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "data.txt")
	os.WriteFile(fpath, []byte("hello"), 0644)

	resources := make(map[string][]byte)
	err := collectEmbedFile(fpath, "data.txt", resources)
	if err != nil {
		t.Fatal(err)
	}
	if string(resources["data.txt"]) != "hello" {
		t.Errorf("content mismatch: got %q", resources["data.txt"])
	}
}

func TestCleanPhysicalPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"template/", "template"},
		{"template", "template"},
		{"template///", "template"},
		{"./", "."},
		{"", "."},
		{"/", "."},
		{"examples/playground/", "examples/playground"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := cleanPhysicalPath(tt.input)
			if got != tt.want {
				t.Errorf("cleanPhysicalPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- helpers ---

func expectKeys(t *testing.T, resources map[string][]byte, expected []string) {
	t.Helper()
	got := mapKeys(resources)
	sort.Strings(got)
	sort.Strings(expected)

	if len(got) != len(expected) {
		t.Errorf("expected %d keys %v, got %d %v", len(expected), expected, len(got), got)
		return
	}
	for i := range expected {
		if got[i] != expected[i] {
			t.Errorf("key[%d]: got %q, want %q\n  all got:  %v\n  all want: %v", i, got[i], expected[i], got, expected)
			return
		}
	}
}

func mapKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Verify content is preserved correctly
func TestCollectEmbedDirContent(t *testing.T) {
	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "assets")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "style.css"), []byte("body { color: red }"), 0644)

	resources := make(map[string][]byte)
	err := collectEmbedDir(dir, "assets", "", resources)
	if err != nil {
		t.Fatal(err)
	}
	if string(resources["assets/style.css"]) != "body { color: red }" {
		t.Errorf("content mismatch: got %q", resources["assets/style.css"])
	}
}

// Verify multiple aliases produce separate keys for the same physical data
func TestMultipleAliasesSameDir(t *testing.T) {
	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "shared")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "data.txt"), []byte("shared data"), 0644)

	resources := make(map[string][]byte)

	// First alias: as "." (flat)
	err := collectEmbedDir(dir, ".", "", resources)
	if err != nil {
		t.Fatal(err)
	}
	// Second alias: as "../views/shared"
	err = collectEmbedDir(dir, "../views/shared", "", resources)
	if err != nil {
		t.Fatal(err)
	}

	// Both keys should exist with identical content
	if string(resources["data.txt"]) != "shared data" {
		t.Errorf("flat key missing or wrong: %q", resources["data.txt"])
	}
	if string(resources["../views/shared/data.txt"]) != "shared data" {
		t.Errorf("aliased key missing or wrong: %q", resources["../views/shared/data.txt"])
	}
}

// End-to-end: parseEmbedArg → collectEmbed*
func TestParseAndCollectIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "tpl")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "a.html"), []byte("A"), 0644)
	os.WriteFile(filepath.Join(dir, "b.js"), []byte("B"), 0644)

	tests := []struct {
		name     string
		arg      string // raw --embed argument (with tplDir substituted)
		wantKeys []string
	}{
		{
			"no alias",
			dir,
			[]string{dir + "/a.html", dir + "/b.js"}, // absolute keys (because arg is absolute)
		},
		{
			"alias dot glob html",
			dir + "/@.@*.html",
			[]string{"a.html"},
		},
		{
			"alias views",
			dir + "@views@",
			[]string{"views/a.html", "views/b.js"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resources := make(map[string][]byte)
			spec := parseEmbedArg(tt.arg)
			physPath := cleanPhysicalPath(spec.PhysicalPath)

			info, err := os.Stat(physPath)
			if err != nil {
				t.Fatal(err)
			}

			if info.IsDir() {
				err = collectEmbedDir(physPath, spec.Alias, spec.GlobFilter, resources)
			} else {
				err = collectEmbedFile(physPath, filepath.ToSlash(spec.PhysicalPath), resources)
			}
			if err != nil {
				t.Fatal(err)
			}

			// For the "no alias" case, keys contain absolute paths — normalize
			if !spec.HasAlias && strings.HasPrefix(spec.Alias, "/") {
				// Keys are absolute; we check by suffix
				for _, wantKey := range tt.wantKeys {
					found := false
					for k := range resources {
						if k == filepath.ToSlash(wantKey) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("missing key %q, have %v", wantKey, mapKeys(resources))
					}
				}
				return
			}

			expectKeys(t, resources, tt.wantKeys)
		})
	}
}
