package ext

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseConfig_ValidMinimal(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/redis/go-redis/v9
    version: v9.7.3
    bind:
      - type: Client
        as: redis
`
	cfg, err := ParseConfig([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Deps) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(cfg.Deps))
	}
	dep := cfg.Deps[0]
	if dep.Pkg != "github.com/redis/go-redis/v9" {
		t.Errorf("pkg = %q, want github.com/redis/go-redis/v9", dep.Pkg)
	}
	if dep.Version != "v9.7.3" {
		t.Errorf("version = %q, want v9.7.3", dep.Version)
	}
	if len(dep.Bind) != 1 {
		t.Fatalf("expected 1 bind, got %d", len(dep.Bind))
	}
	bind := dep.Bind[0]
	if bind.Type != "Client" {
		t.Errorf("type = %q, want Client", bind.Type)
	}
	if bind.As != "redis" {
		t.Errorf("as = %q, want redis", bind.As)
	}
}

func TestParseConfig_ValidBindAll(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/google/uuid
    version: v1.6.0
    bind_all: true
    as: uuid
`
	cfg, err := ParseConfig([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dep := cfg.Deps[0]
	if !dep.BindAll {
		t.Error("expected bind_all to be true")
	}
	if dep.As != "uuid" {
		t.Errorf("as = %q, want uuid", dep.As)
	}
}

func TestParseConfig_ValidFuncBinding(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/go-resty/resty/v2
    version: v2.14.0
    bind:
      - func: New
        as: newHttpClient
`
	cfg, err := ParseConfig([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bind := cfg.Deps[0].Bind[0]
	if bind.Func != "New" {
		t.Errorf("func = %q, want New", bind.Func)
	}
	if bind.As != "newHttpClient" {
		t.Errorf("as = %q, want newHttpClient", bind.As)
	}
}

func TestParseConfig_ValidAdvancedOptions(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/redis/go-redis/v9
    bind:
      - type: Client
        as: redis
        methods: [Get, Set, Del, Ping]
        error_to_result: true
        skip_context: true
        chain_result: Result
`
	cfg, err := ParseConfig([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bind := cfg.Deps[0].Bind[0]
	if !bind.ErrorToResult {
		t.Error("expected error_to_result true")
	}
	if !bind.SkipContext {
		t.Error("expected skip_context true")
	}
	if bind.ChainResult != "Result" {
		t.Errorf("chain_result = %q, want Result", bind.ChainResult)
	}
	if len(bind.Methods) != 4 {
		t.Errorf("methods len = %d, want 4", len(bind.Methods))
	}
}

func TestParseConfig_DefaultVersion(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/redis/go-redis/v9
    bind:
      - type: Client
        as: redis
`
	cfg, err := ParseConfig([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Deps[0].Version != "latest" {
		t.Errorf("default version = %q, want latest", cfg.Deps[0].Version)
	}
}

func TestParseConfig_MultipleDeps(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/redis/go-redis/v9
    version: v9.7.3
    bind:
      - type: Client
        as: redis
        chain_result: Result
  - pkg: github.com/slack-go/slack
    version: v0.15.0
    bind:
      - type: Client
        as: slack
`
	cfg, err := ParseConfig([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Deps) != 2 {
		t.Fatalf("expected 2 deps, got %d", len(cfg.Deps))
	}

	paths := cfg.ExtModulePaths()
	if len(paths) != 2 {
		t.Fatalf("expected 2 ext paths, got %d: %v", len(paths), paths)
	}
	// Module names are derived from pkg path:
	// github.com/redis/go-redis/v9 → "go-redis" (last non-version segment, hyphen kept)
	// github.com/slack-go/slack → "slack"
	if paths[0] != "ext/go-redis" {
		t.Errorf("paths[0] = %q, want ext/go-redis", paths[0])
	}
	if paths[1] != "ext/slack" {
		t.Errorf("paths[1] = %q, want ext/slack", paths[1])
	}
}

// --- Validation error tests ---

func TestParseConfig_ErrorNoDeps(t *testing.T) {
	yaml := `
deps: []
`
	_, err := ParseConfig([]byte(yaml), "test.yaml")
	if err == nil {
		t.Fatal("expected error for empty deps")
	}
}

func TestParseConfig_ErrorNoPkg(t *testing.T) {
	yaml := `
deps:
  - version: v1.0.0
    bind:
      - type: Foo
        as: foo
`
	_, err := ParseConfig([]byte(yaml), "test.yaml")
	if err == nil {
		t.Fatal("expected error for missing pkg")
	}
}

func TestParseConfig_ErrorBindAllAndBind(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/example/pkg
    bind_all: true
    as: ex
    bind:
      - type: Foo
        as: foo
`
	_, err := ParseConfig([]byte(yaml), "test.yaml")
	if err == nil {
		t.Fatal("expected error for bind_all + bind")
	}
}

func TestParseConfig_ErrorBindAllWithoutAs(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/example/pkg
    bind_all: true
`
	_, err := ParseConfig([]byte(yaml), "test.yaml")
	if err == nil {
		t.Fatal("expected error for bind_all without as")
	}
}

func TestParseConfig_ErrorNoBindOrBindAll(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/example/pkg
`
	_, err := ParseConfig([]byte(yaml), "test.yaml")
	if err == nil {
		t.Fatal("expected error for no bind or bind_all")
	}
}

func TestParseConfig_ErrorTypeAndFunc(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/example/pkg
    bind:
      - type: Foo
        func: Bar
        as: baz
`
	_, err := ParseConfig([]byte(yaml), "test.yaml")
	if err == nil {
		t.Fatal("expected error for both type and func")
	}
}

func TestParseConfig_ErrorNoTypeOrFunc(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/example/pkg
    bind:
      - as: baz
`
	_, err := ParseConfig([]byte(yaml), "test.yaml")
	if err == nil {
		t.Fatal("expected error for no type or func")
	}
}

func TestParseConfig_ErrorNoAs(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/example/pkg
    bind:
      - type: Foo
`
	_, err := ParseConfig([]byte(yaml), "test.yaml")
	if err == nil {
		t.Fatal("expected error for missing as")
	}
}

func TestParseConfig_ErrorMethodsOnFunc(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/example/pkg
    bind:
      - func: New
        as: create
        methods: [Foo]
`
	_, err := ParseConfig([]byte(yaml), "test.yaml")
	if err == nil {
		t.Fatal("expected error for methods on func binding")
	}
}

func TestParseConfig_ErrorExcludeMethodsOnFunc(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/example/pkg
    bind:
      - func: New
        as: create
        exclude_methods: [Foo]
`
	_, err := ParseConfig([]byte(yaml), "test.yaml")
	if err == nil {
		t.Fatal("expected error for exclude_methods on func binding")
	}
}

func TestParseConfig_ErrorChainResultOnFunc(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/example/pkg
    bind:
      - func: New
        as: create
        chain_result: Result
`
	_, err := ParseConfig([]byte(yaml), "test.yaml")
	if err == nil {
		t.Fatal("expected error for chain_result on func binding")
	}
}

func TestParseConfig_ValidConstructor(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/redis/go-redis/v9
    bind:
      - type: Options
        as: redisOpts
        constructor: true
`
	cfg, err := ParseConfig([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bind := cfg.Deps[0].Bind[0]
	if !bind.Constructor {
		t.Error("expected constructor true")
	}
}

func TestParseConfig_ErrorConstructorOnFunc(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/example/pkg
    bind:
      - func: New
        as: create
        constructor: true
`
	_, err := ParseConfig([]byte(yaml), "test.yaml")
	if err == nil {
		t.Fatal("expected error for constructor on func binding")
	}
}

func TestParseConfig_ErrorConstructorOnConst(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/example/pkg
    bind:
      - const: MaxSize
        as: maxSize
        constructor: true
`
	_, err := ParseConfig([]byte(yaml), "test.yaml")
	if err == nil {
		t.Fatal("expected error for constructor on const binding")
	}
}

func TestParseConfig_ErrorAliasConflict(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/example/a
    bind:
      - type: Client
        as: cache
  - pkg: github.com/example/b
    bind:
      - type: Client
        as: cache
`
	_, err := ParseConfig([]byte(yaml), "test.yaml")
	if err == nil {
		t.Fatal("expected error for alias conflict")
	}
}

func TestGoModRequires(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/redis/go-redis/v9
    version: v9.7.3
    bind:
      - type: Client
        as: redis
  - pkg: github.com/slack-go/slack
    bind:
      - func: New
        as: slackNew
`
	cfg, err := ParseConfig([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requires := cfg.GoModRequires()
	if len(requires) != 2 {
		t.Fatalf("expected 2 requires, got %d", len(requires))
	}
	if requires[0] != "github.com/redis/go-redis/v9 v9.7.3" {
		t.Errorf("requires[0] = %q", requires[0])
	}
	// No explicit version → "latest" default → pkg path only (no version suffix)
	if requires[1] != "github.com/slack-go/slack" {
		t.Errorf("requires[1] = %q", requires[1])
	}
}

func TestGoModRequires_MonorepoModule(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/aws/aws-sdk-go-v2/aws
    module: github.com/aws/aws-sdk-go-v2
    version: v1.36.3
    bind:
      - type: Config
        as: awsConfig
        constructor: true
  - pkg: github.com/aws/aws-sdk-go-v2/service/s3
    version: v1.78.2
    bind:
      - func: NewFromConfig
        as: s3New
`
	cfg, err := ParseConfig([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// GoModPath: aws subpackage should use module path, s3 should use pkg
	if cfg.Deps[0].GoModPath() != "github.com/aws/aws-sdk-go-v2" {
		t.Errorf("aws GoModPath = %q", cfg.Deps[0].GoModPath())
	}
	if cfg.Deps[1].GoModPath() != "github.com/aws/aws-sdk-go-v2/service/s3" {
		t.Errorf("s3 GoModPath = %q", cfg.Deps[1].GoModPath())
	}

	requires := cfg.GoModRequires()
	if len(requires) != 2 {
		t.Fatalf("expected 2 requires, got %d: %v", len(requires), requires)
	}
	// First dep: module field overrides pkg in go.mod
	if requires[0] != "github.com/aws/aws-sdk-go-v2 v1.36.3" {
		t.Errorf("requires[0] = %q, want module path not pkg", requires[0])
	}
	// Second dep: no module field, pkg used as-is
	if requires[1] != "github.com/aws/aws-sdk-go-v2/service/s3 v1.78.2" {
		t.Errorf("requires[1] = %q", requires[1])
	}
}

func TestFindConfig(t *testing.T) {
	// Create a temp directory structure
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "a", "b", "c")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write funxy.yaml at the top level
	cfgPath := filepath.Join(tmpDir, "funxy.yaml")
	content := `
deps:
  - pkg: github.com/example/pkg
    bind:
      - type: Foo
        as: foo
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// FindConfig from deep subdirectory should find it
	found, err := FindConfig(subDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found != cfgPath {
		t.Errorf("found = %q, want %q", found, cfgPath)
	}

	// FindConfig from a totally different directory should not find it
	otherDir := t.TempDir()
	found, err = FindConfig(otherDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found != "" {
		t.Errorf("expected empty, got %q", found)
	}
}

func TestParseConfig_ExcludeMethods(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/redis/go-redis/v9
    bind:
      - type: Client
        as: redis
        exclude_methods: [Close, Options, PoolStats]
`
	cfg, err := ParseConfig([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bind := cfg.Deps[0].Bind[0]
	if len(bind.ExcludeMethods) != 3 {
		t.Errorf("exclude_methods len = %d, want 3", len(bind.ExcludeMethods))
	}
}

func TestParseConfig_SameAliasSamePackageOK(t *testing.T) {
	// Same alias from the same package should not conflict
	yaml := `
deps:
  - pkg: github.com/redis/go-redis/v9
    bind:
      - type: Client
        as: redis
  - pkg: github.com/redis/go-redis/v9
    bind:
      - type: Options
        as: redis
`
	_, err := ParseConfig([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
