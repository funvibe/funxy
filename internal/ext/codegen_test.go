package ext

import "testing"

func TestImportAlias(t *testing.T) {
	tests := []struct {
		pkgPath  string
		expected string
	}{
		{"net/http", "http"},
		{"github.com/redis/go-redis/v9", "goredis"}, // Fixed case
		{"github.com/redis/go-redis", "goredis"},    // Fixed case
		{"github.com/foo/go", "pkgGo"},              // Reserved word
		{"github.com/foo/map", "pkgMap"},            // Reserved word
		{"context", "pkgContext"},                   // Reserved generated identifier
		{"fmt", "pkgFmt"},                           // Reserved generated identifier
		{"github.com/foo/ext", "pkgExt"},            // Reserved generated identifier
		{"github.com/foo/bar-baz", "barbaz"},        // Hyphen
		{"github.com/foo/bar.baz", "barbaz"},        // Dot
		{"github.com/foo/bar baz", "barbaz"},        // Space
		{"github.com/foo/v9", "foo"},                // Version stripping
		{"v9", "v9"},                                // Edge case: just version
		{"", "pkg"},                                 // Empty
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.pkgPath, func(t *testing.T) {
			t.Parallel()
			got := ImportAlias(tt.pkgPath)
			if got != tt.expected {
				t.Errorf("ImportAlias(%q) = %q; want %q", tt.pkgPath, got, tt.expected)
			}
		})
	}
}
