package ext

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Builder assembles the temporary Go project and builds the custom funxy binary.
type Builder struct {
	// config is the parsed funxy.yaml.
	config *Config

	// funxySourceDir is the path to the Funxy source tree.
	// Used for the replace directive in go.mod. Optional.
	funxySourceDir string

	// funxyModulePath is the Go module path of the Funxy project (e.g. "parser").
	funxyModulePath string

	// funxyVersion is the version or branch to require (e.g. "v1.2.3" or "dev").
	funxyVersion string

	// goVersion is the Go version string (e.g. "1.25.3").
	goVersion string

	// outputPath is the final binary output path.
	outputPath string

	// targetOS is the GOOS for cross-compilation (empty = native).
	targetOS string

	// targetArch is the GOARCH for cross-compilation (empty = native).
	targetArch string

	// workDir is the temporary build directory (reused from Inspector if available).
	workDir string

	// configDir is the directory containing funxy.yaml.
	// Used for resolving local: paths in deps.
	configDir string

	// verbose enables detailed build output.
	verbose bool
}

// BuilderOption configures a Builder.
type BuilderOption func(*Builder)

// WithOutput sets the output binary path.
func WithOutput(path string) BuilderOption {
	return func(b *Builder) { b.outputPath = path }
}

// WithCrossCompile sets the target OS/arch for cross-compilation.
func WithCrossCompile(goos, goarch string) BuilderOption {
	return func(b *Builder) { b.targetOS = goos; b.targetArch = goarch }
}

// WithVerbose enables verbose build output.
func WithVerbose(v bool) BuilderOption {
	return func(b *Builder) { b.verbose = v }
}

// WithWorkDir reuses an existing workspace directory (e.g. from Inspector).
func WithWorkDir(dir string) BuilderOption {
	return func(b *Builder) { b.workDir = dir }
}

// WithConfigDir sets the directory containing funxy.yaml (for local: paths).
func WithConfigDir(dir string) BuilderOption {
	return func(b *Builder) { b.configDir = dir }
}

// NewBuilder creates a new Builder.
//
// funxySourceDir is the path to the Funxy source tree (for replace directive).
// funxyModulePath is the Go module path (e.g. "parser" or "github.com/funvibe/funxy").
// goVersion is the Go version string (e.g. "1.25.3").
func NewBuilder(cfg *Config, funxySourceDir, funxyModulePath, goVersion string, opts ...BuilderOption) *Builder {
	version := "v0.0.0"
	if idx := strings.LastIndex(funxyModulePath, "@"); idx != -1 {
		version = funxyModulePath[idx+1:]
		funxyModulePath = funxyModulePath[:idx]
	}

	b := &Builder{
		config:          cfg,
		funxySourceDir:  funxySourceDir,
		funxyModulePath: funxyModulePath,
		funxyVersion:    version,
		goVersion:       goVersion,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// BuildResult contains the output of a successful build.
type BuildResult struct {
	// BinaryPath is the path to the built binary.
	BinaryPath string

	// WorkDir is the temporary build directory (caller may want to inspect it).
	WorkDir string
}

// Build performs the full build pipeline:
// 1. Setup workspace
// 2. Inspect Go packages
// 3. Generate binding code
// 4. Write helpers and entry file
// 5. Update go.mod
// 6. Build
func (b *Builder) Build() (*BuildResult, error) {
	// Step 1: Setup workspace
	if err := b.ensureWorkDir(); err != nil {
		return nil, fmt.Errorf("workspace setup: %w", err)
	}

	if b.verbose {
		fmt.Fprintf(os.Stderr, "[ext] workspace: %s\n", b.workDir)
	}

	// Step 2: Inspect Go packages
	inspector := NewInspector(b.goVersion)
	// Reuse our workspace
	inspector.workDir = b.workDir

	if err := inspector.setupWorkspace(b.config, b.configDir); err != nil {
		return nil, fmt.Errorf("workspace setup: %w", err)
	}
	// Update workDir in case inspector created a new one
	b.workDir = inspector.WorkDir()

	pkgPaths := collectPackagePaths(b.config)
	if err := inspector.loadPackages(pkgPaths); err != nil {
		return nil, fmt.Errorf("loading packages: %w", err)
	}

	result, err := inspector.Inspect(b.config)
	if err != nil {
		return nil, fmt.Errorf("inspecting packages: %w", err)
	}

	if b.verbose {
		fmt.Fprintf(os.Stderr, "[ext] resolved %d bindings\n", len(result.Bindings))
	}

	// Step 3: Generate binding code
	codegen := NewCodeGenerator(b.funxyModulePath)
	files, err := codegen.Generate(result)
	if err != nil {
		return nil, fmt.Errorf("generating code: %w", err)
	}

	// Step 4: Write helpers
	helpersContent := HelpersTemplate(b.funxyModulePath)
	if err := os.WriteFile(filepath.Join(b.workDir, "ext_helpers.go"), []byte(helpersContent), 0o644); err != nil {
		return nil, fmt.Errorf("writing helpers: %w", err)
	}

	// Step 5: Write generated files
	for _, f := range files {
		path := filepath.Join(b.workDir, f.Filename)
		if err := os.WriteFile(path, []byte(f.Content), 0o644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", f.Filename, err)
		}
		if b.verbose {
			fmt.Fprintf(os.Stderr, "[ext] wrote %s\n", f.Filename)
		}
	}

	// Step 6: Generate main.go entry file
	if err := b.generateEntryFile(); err != nil {
		return nil, fmt.Errorf("generating entry file: %w", err)
	}

	// Step 7: Update go.mod with replace directive and Funxy dependency
	if err := b.updateGoMod(); err != nil {
		return nil, fmt.Errorf("updating go.mod: %w", err)
	}

	// Step 8: Format generated code
	if err := b.goFmt(); err != nil {
		// Non-fatal: formatting errors shouldn't block the build
		if b.verbose {
			fmt.Fprintf(os.Stderr, "[ext] warning: go fmt: %v\n", err)
		}
	}

	// Step 8.5: Run go mod tidy
	if err := b.goModTidy(); err != nil {
		return nil, fmt.Errorf("go mod tidy: %w", err)
	}

	// Step 9: Build
	binaryPath, err := b.goBuild()
	if err != nil {
		return nil, fmt.Errorf("go build: %w", err)
	}

	return &BuildResult{
		BinaryPath: binaryPath,
		WorkDir:    b.workDir,
	}, nil
}

// Cleanup removes the temporary workspace.
func (b *Builder) Cleanup() {
	if b.workDir != "" {
		os.RemoveAll(b.workDir)
		b.workDir = ""
	}
}

func (b *Builder) ensureWorkDir() error {
	if b.workDir != "" {
		return nil
	}
	dir, err := os.MkdirTemp("", "funxy-ext-build-*")
	if err != nil {
		return err
	}
	b.workDir = dir
	return nil
}

// generateEntryFile generates the main.go file that imports pkg/cli and calls cli.Run().
func (b *Builder) generateEntryFile() error {
	content := fmt.Sprintf(`package main

import "%s/pkg/cli"

func main() {
	cli.Run()
}
`, b.funxyModulePath)

	path := filepath.Join(b.workDir, "main.go")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing main.go: %w", err)
	}

	if b.verbose {
		fmt.Fprintf(os.Stderr, "[ext] generated main.go importing %s/pkg/cli\n", b.funxyModulePath)
	}
	return nil
}

// updateGoMod rewrites go.mod to include the replace directive and Funxy dependency.
func (b *Builder) updateGoMod() error {
	// Read current go.mod
	gomodPath := filepath.Join(b.workDir, "go.mod")
	data, err := os.ReadFile(gomodPath)
	if err != nil {
		return fmt.Errorf("reading go.mod: %w", err)
	}

	content := string(data)

	// Add funxy dependency if not already present
	if !strings.Contains(content, b.funxyModulePath) {
		// Add to require block
		if strings.Contains(content, "require (") {
			content = strings.Replace(content, "require (",
				fmt.Sprintf("require (\n\t%s %s", b.funxyModulePath, b.funxyVersion), 1)
		} else {
			// Fallback if "require (" block not found (e.g. single line requires or multiple blocks)
			// Append to end
			content += fmt.Sprintf("\nrequire %s %s\n", b.funxyModulePath, b.funxyVersion)
		}
	}

	// Add replace directive ONLY if source dir is available
	if b.funxySourceDir != "" {
		absSourceDir, err := filepath.Abs(b.funxySourceDir)
		if err != nil {
			return fmt.Errorf("resolving funxy source dir: %w", err)
		}

		if !strings.Contains(content, "replace "+b.funxyModulePath) {
			content += fmt.Sprintf("\nreplace %s => %s\n", b.funxyModulePath, absSourceDir)
		}

		// Also read the Funxy go.mod to carry over its replace directives
		funxyGoMod, err := os.ReadFile(filepath.Join(absSourceDir, "go.mod"))
		if err == nil {
			// Extract replace directives from Funxy's go.mod
			for _, line := range strings.Split(string(funxyGoMod), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "replace ") && !strings.Contains(content, line) {
					// Make relative paths absolute
					parts := strings.Fields(line)
					if len(parts) >= 4 && parts[2] == "=>" {
						replacePath := parts[3]
						if !filepath.IsAbs(replacePath) {
							replacePath = filepath.Join(absSourceDir, replacePath)
						}
						content += fmt.Sprintf("replace %s %s => %s\n", parts[1], "", replacePath)
					}
				}
			}
		}
	}

	if b.verbose {
		fmt.Fprintf(os.Stderr, "[ext] updated go.mod:\n%s\n", content)
	}
	return os.WriteFile(gomodPath, []byte(content), 0o644)
}

// goFmt runs gofmt on all .go files in the workspace.
func (b *Builder) goFmt() error {
	cmd := exec.Command("gofmt", "-w", ".")
	cmd.Dir = b.workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gofmt: %s\n%w", string(output), err)
	}
	return nil
}

// goModTidy runs go mod tidy in the workspace.
func (b *Builder) goModTidy() error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = b.workDir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s\n%w", string(output), err)
	}
	if b.verbose {
		fmt.Fprintf(os.Stderr, "[ext] go mod tidy OK\n")
	}
	return nil
}

// goBuild runs go build and returns the path to the built binary.
func (b *Builder) goBuild() (string, error) {
	outputPath := b.outputPath
	if outputPath == "" {
		outputPath = filepath.Join(b.workDir, "funxy-ext")
		if runtime.GOOS == "windows" || b.targetOS == "windows" {
			outputPath += ".exe"
		}
	}

	args := []string{"build", "-trimpath", "-ldflags=-s -w", "-o", outputPath}

	// Add the workspace directory
	args = append(args, ".")

	cmd := exec.Command("go", args...)
	cmd.Dir = b.workDir

	env := append(os.Environ(), "GOWORK=off", "CGO_ENABLED=0")
	if b.targetOS != "" {
		env = append(env, "GOOS="+b.targetOS)
	}
	if b.targetArch != "" {
		env = append(env, "GOARCH="+b.targetArch)
	}
	cmd.Env = env

	if b.verbose {
		fmt.Fprintf(os.Stderr, "[ext] go build -o %s .\n", outputPath)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("go build failed:\n%s\n%w", string(output), err)
	}

	if b.verbose {
		fmt.Fprintf(os.Stderr, "[ext] go build OK\n")
	}

	return outputPath, nil
}
