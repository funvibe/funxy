package vm

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"path/filepath"

	"github.com/funvibe/funxy/internal/evaluator"
)

func init() {
	// Register bundle types for gob serialization
	gob.Register(&Bundle{})
	gob.Register(&BundledModule{})
	gob.Register(map[string]*BundledModule{})
	gob.Register(map[string]*CompiledFunction{})
	gob.Register(map[string]*Bundle{})
}

// Bundle represents a complete compiled program with all user module dependencies.
// This is the v2 bytecode format that replaces the single-Chunk v1 format.
//
// Single-command mode: MainChunk is set, Commands is nil/empty.
// Multi-command mode:  MainChunk is nil, Commands maps command names to sub-bundles.
//
//	Resources are shared across all commands.
type Bundle struct {
	// MainChunk is the compiled bytecode for the entry script (single-command mode)
	MainChunk *Chunk

	// Modules maps absolute path -> pre-compiled module
	// All user (non-virtual) module dependencies are included
	Modules map[string]*BundledModule

	// TraitDefaults holds pre-compiled trait default methods from the main script
	// Key format: "TraitName.methodName"
	TraitDefaults map[string]*CompiledFunction

	// SourceFile is the original source file path (for error messages)
	SourceFile string

	// Resources holds embedded static files (HTML, images, configs, etc.)
	// Key is the relative path from the source file directory, value is file contents.
	// Populated by --embed flag during build.
	Resources map[string][]byte

	// Commands maps command name -> sub-bundle for multi-command binaries.
	// When set, MainChunk should be nil. Each sub-bundle has its own
	// MainChunk, Modules, and TraitDefaults; Resources are shared from the parent.
	Commands map[string]*Bundle
}

// BundledModule represents a single pre-compiled user module in the bundle.
type BundledModule struct {
	// Chunk is the compiled bytecode for this module (nil for package groups)
	Chunk *Chunk

	// PendingImports are this module's own import dependencies
	PendingImports []PendingImport

	// Exports lists the exported symbol names
	Exports []string

	// TraitDefaults holds pre-compiled trait default methods from this module
	TraitDefaults map[string]*CompiledFunction

	// Dir is the original directory path of this module
	Dir string

	// Traits maps trait names to their method names.
	// Used to resolve `import "mod" (TraitName)` in bundled mode.
	Traits map[string][]string

	// IsPackageGroup is true if this module combines sub-packages
	IsPackageGroup bool

	// SubModulePaths lists absolute paths of sub-modules (only for package groups)
	SubModulePaths []string
}

// bytecodeVersion constants
const (
	bytecodeVersionV1 byte = 0x01 // Single chunk (legacy)
	bytecodeVersionV2 byte = 0x02 // Full bundle with modules
)

// selfContainedMagic is the footer magic for self-contained binaries
var selfContainedMagic = [4]byte{'F', 'X', 'Y', 'S'}

// selfContainedFooterSize is the size of the self-contained footer:
// 8 bytes (bundle size) + 4 bytes (magic)
const selfContainedFooterSize = 12

// SerializeBundle converts a Bundle to binary format.
// Format:
// - Magic number (4 bytes): "FXYB"
// - Version (1 byte): 0x02
// - Gob-encoded Bundle data
func (b *Bundle) Serialize() ([]byte, error) {
	buf := new(bytes.Buffer)

	// Magic number
	buf.Write([]byte{0x46, 0x58, 0x59, 0x42}) // "FXYB"

	// Version
	buf.WriteByte(bytecodeVersionV2)

	// Encode the bundle using gob
	enc := gob.NewEncoder(buf)
	if err := enc.Encode(b); err != nil {
		return nil, fmt.Errorf("bundle gob encoding failed: %w", err)
	}

	return buf.Bytes(), nil
}

// DeserializeAny reads bytecode data and returns either a *Bundle (v2) or wraps
// a legacy *Chunk (v1) into a Bundle for uniform handling.
func DeserializeAny(data []byte) (*Bundle, error) {
	if len(data) < 5 {
		return nil, fmt.Errorf("bytecode data too short")
	}

	// Check magic number
	if data[0] != 0x46 || data[1] != 0x58 || data[2] != 0x59 || data[3] != 0x42 {
		return nil, fmt.Errorf("invalid magic number, expected FXYB")
	}

	version := data[4]
	payload := data[5:]

	switch version {
	case bytecodeVersionV1:
		// Legacy single-chunk format: decode Chunk, wrap in Bundle
		buf := bytes.NewReader(payload)
		dec := gob.NewDecoder(buf)
		var chunk Chunk
		if err := dec.Decode(&chunk); err != nil {
			return nil, fmt.Errorf("v1 gob decoding failed: %w", err)
		}
		return &Bundle{
			MainChunk: &chunk,
			Modules:   make(map[string]*BundledModule),
		}, nil

	case bytecodeVersionV2:
		// Full bundle format
		buf := bytes.NewReader(payload)
		dec := gob.NewDecoder(buf)
		var bundle Bundle
		if err := dec.Decode(&bundle); err != nil {
			return nil, fmt.Errorf("v2 gob decoding failed: %w", err)
		}
		// Ensure maps are initialized
		if bundle.Modules == nil {
			bundle.Modules = make(map[string]*BundledModule)
		}
		if bundle.TraitDefaults == nil {
			bundle.TraitDefaults = make(map[string]*CompiledFunction)
		}
		if bundle.Resources == nil {
			bundle.Resources = make(map[string][]byte)
		}
		if bundle.Commands == nil {
			bundle.Commands = make(map[string]*Bundle)
		}
		if err := bundle.Validate(); err != nil {
			return nil, fmt.Errorf("v2 bundle validation failed: %w", err)
		}
		return &bundle, nil

	default:
		return nil, fmt.Errorf(
			"unsupported bytecode version: %d (this binary supports versions %d–%d; upgrade Funxy to run newer bytecode)",
			version, bytecodeVersionV1, bytecodeVersionV2)
	}
}

// PackSelfContained creates a self-contained binary by appending bundle data
// to the host binary with a footer.
// Output format: [hostBinary][bundleData][8-byte bundleSize LE][4-byte "FXYS"]
func PackSelfContained(hostBinary []byte, bundle *Bundle) ([]byte, error) {
	bundleData, err := bundle.Serialize()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize bundle: %w", err)
	}

	totalSize := len(hostBinary) + len(bundleData) + selfContainedFooterSize
	out := make([]byte, 0, totalSize)
	out = append(out, hostBinary...)
	out = append(out, bundleData...)

	// Write footer: 8-byte bundle size (little-endian)
	sizeBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(sizeBuf, uint64(len(bundleData)))
	out = append(out, sizeBuf...)

	// Write footer magic
	out = append(out, selfContainedMagic[:]...)

	return out, nil
}

// ExtractEmbeddedBundle reads a self-contained binary and extracts the embedded
// bundle, if present. Returns nil, nil if no embedded bundle is found.
func ExtractEmbeddedBundle(binaryData []byte) (*Bundle, error) {
	size := len(binaryData)
	if size < selfContainedFooterSize {
		return nil, nil // Too small, no bundle
	}

	// Check footer magic
	footerStart := size - selfContainedFooterSize
	magic := binaryData[size-4:]
	if magic[0] != selfContainedMagic[0] || magic[1] != selfContainedMagic[1] ||
		magic[2] != selfContainedMagic[2] || magic[3] != selfContainedMagic[3] {
		return nil, nil // No magic, not a self-contained binary
	}

	// Read bundle size
	bundleSize := binary.LittleEndian.Uint64(binaryData[footerStart : footerStart+8])

	// Validate
	if bundleSize == 0 || int64(bundleSize) > int64(footerStart) {
		return nil, fmt.Errorf("invalid embedded bundle size: %d", bundleSize)
	}

	bundleStart := int64(footerStart) - int64(bundleSize)
	bundleData := binaryData[bundleStart:footerStart]

	return DeserializeAny(bundleData)
}

// GetHostBinarySize returns the size of the host binary portion of a self-contained
// binary (i.e., the size WITHOUT the appended bundle data and footer).
// Strips all layers of appended bundles (handles double-pack scenarios).
// Returns the full file size if no embedded bundle is detected.
func GetHostBinarySize(binaryData []byte) int64 {
	size := int64(len(binaryData))

	for size >= selfContainedFooterSize {
		footerStart := size - selfContainedFooterSize
		magic := binaryData[footerStart+8:]
		if magic[0] != selfContainedMagic[0] || magic[1] != selfContainedMagic[1] ||
			magic[2] != selfContainedMagic[2] || magic[3] != selfContainedMagic[3] {
			break // No magic — reached the real host binary
		}

		bundleSize := int64(binary.LittleEndian.Uint64(binaryData[footerStart : footerStart+8]))
		if bundleSize == 0 || bundleSize > footerStart {
			break // Invalid size — stop stripping
		}

		size = footerStart - bundleSize
	}

	return size
}

// IsMultiCommand returns true if this bundle contains multiple named commands.
func (b *Bundle) IsMultiCommand() bool {
	return len(b.Commands) > 0
}

// CommandNames returns sorted list of available command names.
func (b *Bundle) CommandNames() []string {
	names := make([]string, 0, len(b.Commands))
	for name := range b.Commands {
		names = append(names, name)
	}
	// Sort for stable output
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[i] > names[j] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	return names
}

// Validate checks the structural integrity of a deserialized bundle.
func (b *Bundle) Validate() error {
	isMultiCommand := len(b.Commands) > 0

	if isMultiCommand {
		// Multi-command mode: each sub-bundle must have a MainChunk
		for name, cmd := range b.Commands {
			if cmd.MainChunk == nil {
				return fmt.Errorf("command %q has nil MainChunk", name)
			}
			if len(cmd.MainChunk.Code) == 0 {
				return fmt.Errorf("command %q has empty bytecode", name)
			}
		}
	} else {
		// Single-command mode: MainChunk is required
		if b.MainChunk == nil {
			return fmt.Errorf("single-command bundle has nil MainChunk")
		}
		if len(b.MainChunk.Code) == 0 {
			return fmt.Errorf("single-command bundle has empty bytecode")
		}
	}

	return nil
}

// ResolveCommand finds the sub-bundle for a given command name.
// It also inherits shared Resources from the parent bundle.
func (b *Bundle) ResolveCommand(name string) *Bundle {
	cmd, ok := b.Commands[name]
	if !ok {
		return nil
	}
	// Inherit shared resources from parent (sub-bundles don't have their own).
	// Copy the map to avoid shared mutation between parent and child bundles.
	if len(cmd.Resources) == 0 && len(b.Resources) > 0 {
		cmd.Resources = make(map[string][]byte, len(b.Resources))
		for k, v := range b.Resources {
			cmd.Resources[k] = v
		}
	}
	return cmd
}

// --- Helper: run a bundle on a fresh VM ---

// RunBundle creates a VM and executes a bundle. Errors are returned, not printed.
func RunBundle(bundle *Bundle) (evaluator.Object, error) {
	machine := New()
	machine.RegisterBuiltins()
	machine.RegisterFPTraits()
	machine.SetBundle(bundle)

	// Set base dir for resolving relative imports (must match bundle.Modules keys)
	sourceDir := ""
	if bundle.SourceFile != "" {
		sourceDir = filepath.Dir(bundle.SourceFile)
	} else if bundle.MainChunk != nil && bundle.MainChunk.File != "" {
		sourceDir = filepath.Dir(bundle.MainChunk.File)
	}
	if sourceDir != "" {
		machine.SetBaseDir(sourceDir)
	}

	// Set file info for error messages
	if bundle.SourceFile != "" {
		machine.SetCurrentFile(bundle.SourceFile)
	}
	if bundle.MainChunk.File != "" {
		machine.SetCurrentFile(bundle.MainChunk.File)
	}

	// Set pre-compiled trait defaults from main script
	if bundle.TraitDefaults != nil {
		machine.compiledTraitDefaults = bundle.TraitDefaults
	}

	// Process imports
	if len(bundle.MainChunk.PendingImports) > 0 {
		if err := machine.ProcessImports(bundle.MainChunk.PendingImports); err != nil {
			return nil, fmt.Errorf("import error: %w", err)
		}
	}

	// Execute main chunk
	result, err := machine.Run(bundle.MainChunk)
	if err != nil {
		return nil, err
	}

	return result, nil
}
