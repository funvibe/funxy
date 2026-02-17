// Package ext implements the Go ecosystem integration for Funxy.
//
// It provides the infrastructure for declaring Go dependencies in funxy.yaml,
// generating Go bindings, and building custom Funxy binaries with Go libraries
// compiled in.
//
// The ext package handles:
//   - Parsing and validating funxy.yaml configuration
//   - Introspecting Go packages via go/packages
//   - Generating Go binding code (ext/*.go)
//   - Generating Funxy type stubs (.d.lang) for LSP
//   - Building custom host binaries with go build
package ext

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the top-level funxy.yaml configuration.
type Config struct {
	// Deps lists the Go package dependencies and their binding specifications.
	Deps []Dep `yaml:"deps"`
}

// Dep represents a single Go package dependency.
type Dep struct {
	// Pkg is the Go import path (e.g. "github.com/redis/go-redis/v9").
	Pkg string `yaml:"pkg"`

	// Version is the Go module version constraint (e.g. "v9.7.3", "latest").
	// Defaults to "latest" if omitted. Ignored when Local is set.
	Version string `yaml:"version,omitempty"`

	// Module is the Go module path for go.mod, when it differs from Pkg.
	// Most packages don't need this — Pkg is used as the module path by default.
	// Required for monorepo packages where the import path (Pkg) differs
	// from the module path. For example, AWS SDK v2:
	//
	//   - pkg: github.com/aws/aws-sdk-go-v2/aws       # import path
	//     module: github.com/aws/aws-sdk-go-v2         # module in go.mod
	//     version: v1.36.3
	Module string `yaml:"module,omitempty"`

	// Local is a path to a local Go package directory (relative to funxy.yaml).
	// When set, a `replace` directive is added to go.mod and the package is
	// not downloaded from the network. The directory must contain valid Go
	// source files with a go.mod (or be part of a Go module).
	//
	// Example:
	//   - pkg: myproject/utils
	//     local: ./golib/utils
	//     bind:
	//       - func: Hash
	//         as: utilsHash
	Local string `yaml:"local,omitempty"`

	// Bind lists the types and functions to bind from this package.
	// Mutually exclusive with BindAll.
	Bind []BindSpec `yaml:"bind,omitempty"`

	// BindAll binds all exported types and functions from the package.
	// Mutually exclusive with Bind.
	BindAll bool `yaml:"bind_all,omitempty"`

	// As is the module alias used in Funxy scripts (e.g. "redis", "s3").
	// Required when BindAll is true. For Bind entries, each entry has its own alias.
	As string `yaml:"as,omitempty"`
}

// BindSpec describes what to bind from a Go type or function.
type BindSpec struct {
	// Type is the Go type name to bind (e.g. "Client"). Methods of this type
	// are exposed as Funxy functions. Mutually exclusive with Func and Const.
	Type string `yaml:"type,omitempty"`

	// Func is a standalone Go function name to bind (e.g. "Parse").
	// Mutually exclusive with Type and Const.
	Func string `yaml:"func,omitempty"`

	// Const is a package-level constant name to bind (e.g. "StatusOK").
	// Mutually exclusive with Type and Func. The constant is exposed
	// as a value (not a function) in the ext module.
	Const string `yaml:"const,omitempty"`

	// As is the Funxy name prefix for this binding (e.g. "redis" → redisGet, redisSet).
	// For Type bindings, methods become <as><MethodName> in camelCase.
	// For Func bindings, this is the exact Funxy function name.
	As string `yaml:"as"`

	// Methods is an optional whitelist of method names to bind.
	// If empty, all exported methods are bound. Only valid with Type.
	Methods []string `yaml:"methods,omitempty"`

	// ExcludeMethods is an optional blacklist of method names to skip.
	// Only valid with Type.
	ExcludeMethods []string `yaml:"exclude_methods,omitempty"`

	// ErrorToResult converts Go (T, error) returns to Funxy Result<String, T>.
	// Defaults to false.
	ErrorToResult bool `yaml:"error_to_result,omitempty"`

	// SkipContext automatically provides context.Background() for methods
	// that accept context.Context as the first parameter.
	// Defaults to false.
	SkipContext bool `yaml:"skip_context,omitempty"`

	// ChainResult calls .Result() (or a custom method) on the return value
	// before converting to Funxy. Useful for go-redis style APIs where
	// methods return a command object with a .Result() method.
	ChainResult string `yaml:"chain_result,omitempty"`

	// Constructor generates a constructor function that creates a Go struct
	// from a Funxy record. The function name is the `as` prefix itself.
	// Only valid with Type (and the type must be a struct).
	// Example: type: Options, as: opts, constructor: true → opts({ addr: "...", db: 0 })
	Constructor bool `yaml:"constructor,omitempty"`

	// TypeArgs specifies explicit Go type arguments for binding generic functions or types.
	// Required for constrained type params (e.g. comparable). For unconstrained (any) params,
	// type arguments are inferred automatically.
	// Example: type_args: [string] → Contains[string](...)
	TypeArgs []string `yaml:"type_args,omitempty"`
}

// LoadConfig reads and parses a funxy.yaml file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	return ParseConfig(data, path)
}

// ParseConfig parses funxy.yaml content from bytes.
// The path argument is used only for error messages.
func ParseConfig(data []byte, path string) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if err := cfg.validate(path); err != nil {
		return nil, err
	}
	cfg.setDefaults()
	return &cfg, nil
}

// FindConfig searches for funxy.yaml starting from dir and walking up
// to parent directories, similar to how .gitignore is found.
// Returns the path to the config file and nil error if found,
// or empty string and nil error if not found.
func FindConfig(dir string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolving directory: %w", err)
	}

	for {
		candidate := filepath.Join(dir, "funxy.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		// Also check funxy.yml (common alternative)
		candidate = filepath.Join(dir, "funxy.yml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			return "", nil
		}
		dir = parent
	}
}

// validate checks the configuration for semantic errors.
func (c *Config) validate(path string) error {
	if len(c.Deps) == 0 {
		return fmt.Errorf("%s: no deps defined", path)
	}

	seenAliases := make(map[string]string) // alias → pkg (for conflict detection)

	configDir := filepath.Dir(path)

	for i, dep := range c.Deps {
		if dep.Pkg == "" {
			return fmt.Errorf("%s: deps[%d]: pkg is required", path, i)
		}

		// Validate local path if specified
		if dep.Local != "" {
			localPath := dep.Local
			if !filepath.IsAbs(localPath) {
				localPath = filepath.Join(configDir, localPath)
			}
			info, err := os.Stat(localPath)
			if err != nil {
				return fmt.Errorf("%s: deps[%d] (%s): local path %q not found: %w",
					path, i, dep.Pkg, dep.Local, err)
			}
			if !info.IsDir() {
				return fmt.Errorf("%s: deps[%d] (%s): local path %q is not a directory",
					path, i, dep.Pkg, dep.Local)
			}
		}

		if dep.BindAll && len(dep.Bind) > 0 {
			return fmt.Errorf("%s: deps[%d] (%s): bind_all and bind are mutually exclusive", path, i, dep.Pkg)
		}

		if dep.BindAll && dep.As == "" {
			return fmt.Errorf("%s: deps[%d] (%s): as is required when bind_all is true", path, i, dep.Pkg)
		}

		if !dep.BindAll && len(dep.Bind) == 0 {
			return fmt.Errorf("%s: deps[%d] (%s): either bind or bind_all is required", path, i, dep.Pkg)
		}

		for j, bind := range dep.Bind {
			specCount := 0
			if bind.Type != "" {
				specCount++
			}
			if bind.Func != "" {
				specCount++
			}
			if bind.Const != "" {
				specCount++
			}
			if specCount == 0 {
				return fmt.Errorf("%s: deps[%d].bind[%d] (%s): one of type, func, or const is required",
					path, i, j, dep.Pkg)
			}
			if specCount > 1 {
				return fmt.Errorf("%s: deps[%d].bind[%d] (%s): type, func, and const are mutually exclusive",
					path, i, j, dep.Pkg)
			}
			if bind.As == "" {
				return fmt.Errorf("%s: deps[%d].bind[%d] (%s): as is required",
					path, i, j, dep.Pkg)
			}
			if bind.Const != "" && (len(bind.Methods) > 0 || len(bind.ExcludeMethods) > 0 || bind.ChainResult != "" || bind.ErrorToResult || bind.SkipContext || bind.Constructor || len(bind.TypeArgs) > 0) {
				return fmt.Errorf("%s: deps[%d].bind[%d] (%s): const bindings only support 'as'",
					path, i, j, dep.Pkg)
			}
			if bind.Func != "" && bind.Constructor {
				return fmt.Errorf("%s: deps[%d].bind[%d] (%s): constructor is only valid with type, not func",
					path, i, j, dep.Pkg)
			}
			if bind.Func != "" && len(bind.Methods) > 0 {
				return fmt.Errorf("%s: deps[%d].bind[%d] (%s): methods is only valid with type, not func",
					path, i, j, dep.Pkg)
			}
			if bind.Func != "" && len(bind.ExcludeMethods) > 0 {
				return fmt.Errorf("%s: deps[%d].bind[%d] (%s): exclude_methods is only valid with type, not func",
					path, i, j, dep.Pkg)
			}
			if bind.Func != "" && bind.ChainResult != "" {
				return fmt.Errorf("%s: deps[%d].bind[%d] (%s): chain_result is only valid with type, not func",
					path, i, j, dep.Pkg)
			}

			// Check alias conflicts
			alias := bind.As
			if prev, ok := seenAliases[alias]; ok && prev != dep.Pkg {
				return fmt.Errorf("%s: deps[%d].bind[%d]: alias %q conflicts with %s",
					path, i, j, alias, prev)
			}
			seenAliases[alias] = dep.Pkg
		}

		// Check bind_all alias conflicts
		if dep.BindAll && dep.As != "" {
			if prev, ok := seenAliases[dep.As]; ok && prev != dep.Pkg {
				return fmt.Errorf("%s: deps[%d]: alias %q conflicts with %s",
					path, i, dep.As, prev)
			}
			seenAliases[dep.As] = dep.Pkg
		}
	}

	return nil
}

// setDefaults fills in default values for omitted fields.
func (c *Config) setDefaults() {
	for i := range c.Deps {
		if c.Deps[i].Version == "" && !c.Deps[i].IsLocal() {
			c.Deps[i].Version = "latest"
		}
	}
}

// IsLocal returns true if this dependency points to a local directory.
func (dep *Dep) IsLocal() bool {
	return dep.Local != ""
}

// GoModPath returns the Go module path for go.mod require.
// Uses Module if set, otherwise falls back to Pkg.
func (dep *Dep) GoModPath() string {
	if dep.Module != "" {
		return dep.Module
	}
	return dep.Pkg
}

// ExtModuleName returns the ext module name for a dependency.
// This is dep.As if set, otherwise derived from the package path.
func (dep *Dep) ExtModuleName() string {
	if dep.As != "" {
		return dep.As
	}
	// Derive from package path: use last non-version segment
	parts := strings.Split(dep.Pkg, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		seg := parts[i]
		// Skip version segments like "v9", "v2"
		if len(seg) > 1 && seg[0] == 'v' {
			allDigits := true
			for _, c := range seg[1:] {
				if c < '0' || c > '9' {
					allDigits = false
					break
				}
			}
			if allDigits {
				continue
			}
		}
		return seg
	}
	return parts[len(parts)-1]
}

// ExtModulePaths returns the list of ext/* module paths that scripts
// can import (e.g. ["ext/redis", "ext/s3"]).
func (c *Config) ExtModulePaths() []string {
	seen := make(map[string]bool)
	var paths []string

	for i := range c.Deps {
		name := c.Deps[i].ExtModuleName()
		p := "ext/" + name
		if !seen[p] {
			paths = append(paths, p)
			seen[p] = true
		}
	}

	return paths
}

// GoModRequires returns the list of Go module requires for go.mod generation.
// Each entry is "pkg version" (e.g. "github.com/redis/go-redis/v9 v9.7.3").
// Local deps get version v0.0.0 (resolved via replace directive).
// Uses GoModPath() so monorepo packages (where module != pkg) work correctly.
func (c *Config) GoModRequires() []string {
	seen := make(map[string]bool)
	var requires []string

	for _, dep := range c.Deps {
		modPath := dep.GoModPath()
		if !seen[modPath] {
			if dep.IsLocal() {
				requires = append(requires, dep.Pkg+" v0.0.0")
			} else {
				version := dep.Version
				if version == "latest" {
					version = "" // go get will resolve to latest
				}
				requires = append(requires, strings.TrimSpace(modPath+" "+version))
			}
			seen[modPath] = true
		}
	}

	return requires
}

// GoModReplaces returns replace directives for local deps.
// Each entry is "pkg => absolute_path".
// configDir is the directory containing funxy.yaml (for resolving relative paths).
func (c *Config) GoModReplaces(configDir string) []string {
	seen := make(map[string]bool)
	var replaces []string

	for _, dep := range c.Deps {
		if dep.IsLocal() && !seen[dep.Pkg] {
			localPath := dep.Local
			if !filepath.IsAbs(localPath) {
				localPath = filepath.Join(configDir, localPath)
			}
			// Clean and resolve to absolute
			absPath, err := filepath.Abs(localPath)
			if err == nil {
				localPath = absPath
			}
			replaces = append(replaces, dep.Pkg+" => "+localPath)
			seen[dep.Pkg] = true
		}
	}

	return replaces
}
