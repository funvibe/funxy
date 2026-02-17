package ext

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Cache manages the ext host binary cache in .funxy/ext-cache/.
// The cache key is a hash of funxy.yaml contents + GOOS + GOARCH,
// so the cached binary is reused when deps haven't changed.
type Cache struct {
	// projectDir is the root directory containing funxy.yaml.
	projectDir string
}

// NewCache creates a new cache scoped to the given project directory.
func NewCache(projectDir string) *Cache {
	return &Cache{projectDir: projectDir}
}

// CacheDir returns the path to the cache directory.
func (c *Cache) CacheDir() string {
	return filepath.Join(c.projectDir, ".funxy", "ext-cache")
}

// LookupHostBinary checks if a cached host binary exists for the given config.
// Returns the path to the binary if found, or empty string if not cached.
func (c *Cache) LookupHostBinary(configData []byte, funxyModPath, targetOS, targetArch string) string {
	key := c.computeKey(configData, funxyModPath, targetOS, targetArch)
	binaryPath := filepath.Join(c.CacheDir(), "host-"+key)

	if targetOS == "windows" || (targetOS == "" && runtime.GOOS == "windows") {
		binaryPath += ".exe"
	}

	info, err := os.Stat(binaryPath)
	if err != nil || info.IsDir() {
		return ""
	}

	// Verify it's executable (non-zero size)
	if info.Size() == 0 {
		os.Remove(binaryPath)
		return ""
	}

	return binaryPath
}

// StoreHostBinary copies a built host binary to the cache.
func (c *Cache) StoreHostBinary(binaryPath string, configData []byte, funxyModPath, targetOS, targetArch string) (string, error) {
	if err := os.MkdirAll(c.CacheDir(), 0o755); err != nil {
		return "", fmt.Errorf("creating cache dir: %w", err)
	}

	key := c.computeKey(configData, funxyModPath, targetOS, targetArch)
	cachedPath := filepath.Join(c.CacheDir(), "host-"+key)

	if targetOS == "windows" || (targetOS == "" && runtime.GOOS == "windows") {
		cachedPath += ".exe"
	}

	// Read the built binary
	data, err := os.ReadFile(binaryPath)
	if err != nil {
		return "", fmt.Errorf("reading binary: %w", err)
	}

	// Write to cache
	if err := os.WriteFile(cachedPath, data, 0o755); err != nil {
		return "", fmt.Errorf("writing cache: %w", err)
	}

	return cachedPath, nil
}

// Clean removes all cached binaries.
func (c *Cache) Clean() error {
	return os.RemoveAll(c.CacheDir())
}

// computeKey generates a deterministic cache key from the config content and target platform.
func (c *Cache) computeKey(configData []byte, funxyModPath, targetOS, targetArch string) string {
	if targetOS == "" {
		targetOS = runtime.GOOS
	}
	if targetArch == "" {
		targetArch = runtime.GOARCH
	}

	h := sha256.New()
	h.Write(configData)
	h.Write([]byte("\x00"))
	h.Write([]byte(targetOS))
	h.Write([]byte("\x00"))
	h.Write([]byte(targetArch))
	h.Write([]byte("\x00"))
	h.Write([]byte(funxyModPath)) // Include module path/version in cache key

	// Include the version of the codegen (so cache invalidates on updates)
	h.Write([]byte("\x00"))
	h.Write([]byte(codegenVersion))

	return hex.EncodeToString(h.Sum(nil))[:16] // First 16 hex chars = 64 bits
}

// codegenVersion is bumped when the generated code format changes.
// This ensures stale cached binaries are rebuilt.
const codegenVersion = "v1"

// CachedBuild performs a build, using the cache when possible.
// Returns the path to the host binary (either cached or freshly built).
func CachedBuild(cfg *Config, configData []byte, projectDir, funxySourceDir, funxyModPath, goVersion string, targetOS, targetArch string, verbose bool) (string, func(), error) {
	cache := NewCache(projectDir)

	// Check cache
	if cached := cache.LookupHostBinary(configData, funxyModPath, targetOS, targetArch); cached != "" {
		if verbose {
			fmt.Fprintf(os.Stderr, "[ext] using cached host binary: %s\n", cached)
		}
		return cached, func() {}, nil // No cleanup needed for cached binary
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[ext] cache miss — building ext host binary\n")
	}

	// Build
	var opts []BuilderOption
	if targetOS != "" || targetArch != "" {
		opts = append(opts, WithCrossCompile(targetOS, targetArch))
	}
	opts = append(opts, WithVerbose(verbose))
	opts = append(opts, WithConfigDir(projectDir))

	builder := NewBuilder(cfg, funxySourceDir, funxyModPath, goVersion, opts...)

	result, err := builder.Build()
	if err != nil {
		builder.Cleanup()
		return "", nil, fmt.Errorf("building host binary: %w", err)
	}

	// Store in cache
	cachedPath, cacheErr := cache.StoreHostBinary(result.BinaryPath, configData, funxyModPath, targetOS, targetArch)
	if cacheErr != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "[ext] warning: failed to cache binary: %v\n", cacheErr)
		}
		// Fall back to the temp binary
		return result.BinaryPath, builder.Cleanup, nil
	}

	// Clean up temp build dir (we have the cached copy now)
	builder.Cleanup()

	return cachedPath, func() {}, nil
}

// ConfigFingerprint returns the raw config file data for cache key computation.
// It normalizes the content by trimming whitespace, so trivial whitespace
// changes don't invalidate the cache.
func ConfigFingerprint(configPath string) ([]byte, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	// Normalize: trim trailing whitespace on each line and trailing newlines
	lines := strings.Split(string(data), "\n")
	var normalized strings.Builder
	for _, line := range lines {
		normalized.WriteString(strings.TrimRight(line, " \t\r"))
		normalized.WriteString("\n")
	}

	return []byte(strings.TrimRight(normalized.String(), "\n")), nil
}
