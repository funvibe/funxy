package vm

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"strings"
	"testing"

	"github.com/funvibe/funxy/internal/evaluator"
)

func TestBundle_SerializeDeserializeRoundtrip(t *testing.T) {
	// 5.1 Serialize → DeserializeAny roundtrip
	chunk := &Chunk{
		Code:           []byte{byte(OP_CONST), 0, 0, byte(OP_HALT)},
		Constants:      []evaluator.Object{},
		Lines:          []int{0, 1},
		File:           "test.lang",
		PendingImports: []PendingImport{{Path: "./foo", Symbols: []string{"bar"}}},
	}

	bundle := &Bundle{
		MainChunk: chunk,
		Modules: map[string]*BundledModule{
			"/abs/path/mod": {
				Chunk:   &Chunk{Code: []byte{byte(OP_HALT)}},
				Exports: []string{"x"},
			},
		},
		Resources: map[string][]byte{
			"data.txt": []byte("hello"),
		},
		TraitDefaults: map[string]*CompiledFunction{},
		SourceFile:    "test.lang",
	}

	data, err := bundle.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	restored, err := DeserializeAny(data)
	if err != nil {
		t.Fatalf("DeserializeAny failed: %v", err)
	}

	if restored.MainChunk == nil {
		t.Error("MainChunk is nil")
	}
	if len(restored.MainChunk.Code) != len(bundle.MainChunk.Code) {
		t.Errorf("MainChunk.Code length: got %d, want %d", len(restored.MainChunk.Code), len(bundle.MainChunk.Code))
	}
	if restored.MainChunk.File != bundle.MainChunk.File {
		t.Errorf("MainChunk.File: got %q, want %q", restored.MainChunk.File, bundle.MainChunk.File)
	}
	if len(restored.Modules) != len(bundle.Modules) {
		t.Errorf("Modules count: got %d, want %d", len(restored.Modules), len(bundle.Modules))
	}
	if len(restored.Resources) != len(bundle.Resources) {
		t.Errorf("Resources count: got %d, want %d", len(restored.Resources), len(bundle.Resources))
	}
	if restored.Resources["data.txt"] == nil || string(restored.Resources["data.txt"]) != "hello" {
		t.Errorf("Resources[\"data.txt\"]: got %q", restored.Resources["data.txt"])
	}
}

func TestDeserializeAny_TooShort(t *testing.T) {
	// 5.2 DeserializeAny — too short
	_, err := DeserializeAny([]byte{0x01})
	if err == nil {
		t.Error("Expected error for too short data")
	}
	if err != nil && !contains(err.Error(), "too short") {
		t.Errorf("Expected 'too short' in error, got: %v", err)
	}
}

func TestDeserializeAny_InvalidMagic(t *testing.T) {
	// 5.3 DeserializeAny — wrong magic
	data := []byte{0x00, 0x00, 0x00, 0x00, 0x02, 0x00}
	_, err := DeserializeAny(data)
	if err == nil {
		t.Error("Expected error for invalid magic")
	}
	if err != nil && !contains(err.Error(), "magic") {
		t.Errorf("Expected 'magic' in error, got: %v", err)
	}
}

func TestDeserializeAny_UnknownVersion(t *testing.T) {
	// 5.4 DeserializeAny — unknown version
	data := []byte{0x46, 0x58, 0x59, 0x42, 0xFF, 0x00} // FXYB + version 0xFF
	_, err := DeserializeAny(data)
	if err == nil {
		t.Error("Expected error for unknown version")
	}
	if err != nil && !contains(err.Error(), "version") {
		t.Errorf("Expected 'version' in error, got: %v", err)
	}
}

func TestDeserializeAny_CorruptedGob(t *testing.T) {
	// 5.5 DeserializeAny — corrupted gob data
	data := []byte{0x46, 0x58, 0x59, 0x42, 0x02}                 // FXYB + v2
	data = append(data, []byte{0x00, 0x01, 0x02, 0xff, 0xfe}...) // garbage
	_, err := DeserializeAny(data)
	if err == nil {
		t.Error("Expected error for corrupted gob")
	}
}

func TestPackSelfContained_ExtractRoundtrip(t *testing.T) {
	// 5.6 PackSelfContained → ExtractEmbeddedBundle roundtrip
	host := []byte("fake host binary data")
	bundle := &Bundle{
		MainChunk: &Chunk{Code: []byte{byte(OP_HALT)}},
		Resources: map[string][]byte{"a.txt": []byte("hello")},
	}

	packed, err := PackSelfContained(host, bundle)
	if err != nil {
		t.Fatalf("PackSelfContained failed: %v", err)
	}

	extracted, err := ExtractEmbeddedBundle(packed)
	if err != nil {
		t.Fatalf("ExtractEmbeddedBundle failed: %v", err)
	}
	if extracted == nil {
		t.Fatal("ExtractEmbeddedBundle returned nil")
	}
	if string(extracted.Resources["a.txt"]) != "hello" {
		t.Errorf("Resources[\"a.txt\"]: got %q, want %q", extracted.Resources["a.txt"], "hello")
	}
}

func TestExtractEmbeddedBundle_NoMagic(t *testing.T) {
	// 5.7 ExtractEmbeddedBundle — no magic
	extracted, err := ExtractEmbeddedBundle([]byte("just a regular binary"))
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if extracted != nil {
		t.Error("Expected nil bundle when no magic")
	}
}

func TestExtractEmbeddedBundle_CorruptedFooterSize(t *testing.T) {
	// 5.8 ExtractEmbeddedBundle — corrupted footer size
	// Create minimum valid footer: 12 bytes at end
	// Magic on place, but bundleSize > fileSize causes error
	host := []byte("host")
	data := append(host, make([]byte, 20)...) // small payload
	// Footer: 8 bytes size (use huge value) + 4 bytes FXYS
	footer := make([]byte, 12)
	// size = 999999 (way larger than payload)
	footer[0] = 0xFF
	footer[1] = 0xFF
	footer[2] = 0x0F
	footer[3] = 0x00
	footer[4] = 0x00
	footer[5] = 0x00
	footer[6] = 0x00
	footer[7] = 0x00
	footer[8] = 'F'
	footer[9] = 'X'
	footer[10] = 'Y'
	footer[11] = 'S'
	packed := append(data, footer...)

	_, err := ExtractEmbeddedBundle(packed)
	if err == nil {
		t.Error("Expected error for corrupted footer size")
	}
}

func TestGetHostBinarySize(t *testing.T) {
	// 5.9 GetHostBinarySize
	host := []byte("fake host binary data")
	bundle := &Bundle{MainChunk: &Chunk{Code: []byte{byte(OP_HALT)}}}
	packed, err := PackSelfContained(host, bundle)
	if err != nil {
		t.Fatalf("PackSelfContained failed: %v", err)
	}
	hostSize := GetHostBinarySize(packed)
	if hostSize != int64(len(host)) {
		t.Errorf("GetHostBinarySize: got %d, want %d", hostSize, len(host))
	}
}

func TestGetHostBinarySize_RegularBinary(t *testing.T) {
	// 5.10 GetHostBinarySize — regular binary (not self-contained)
	data := []byte("no bundle")
	hostSize := GetHostBinarySize(data)
	if hostSize != int64(len(data)) {
		t.Errorf("GetHostBinarySize on regular binary: got %d, want %d", hostSize, len(data))
	}
}

func TestBundle_NilMaps(t *testing.T) {
	// 5.11 Bundle with nil maps → Serialize → DeserializeAny → Maps initialized
	chunk := &Chunk{Code: []byte{byte(OP_HALT)}, Constants: []evaluator.Object{}}
	bundle := &Bundle{
		MainChunk:     chunk,
		Modules:       nil,
		Resources:     nil,
		TraitDefaults: nil,
	}

	data, err := bundle.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	// Use v2 format - need proper gob encoding. Nil maps might encode as nil.
	// DeserializeAny for v2 initializes nil maps
	restored, err := DeserializeAny(data)
	if err != nil {
		t.Fatalf("DeserializeAny failed: %v", err)
	}
	if restored.Modules == nil {
		t.Error("Modules should be initialized (not nil)")
	}
	if restored.Resources == nil {
		t.Error("Resources should be initialized (not nil)")
	}
	if restored.TraitDefaults == nil {
		t.Error("TraitDefaults should be initialized (not nil)")
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}

// --- Multi-command bundle tests ---

func TestBundle_MultiCommand_SerializeDeserialize(t *testing.T) {
	// Create a multi-command bundle with two commands
	apiChunk := &Chunk{
		Code:      []byte{byte(OP_CONST), 0, 0, byte(OP_HALT)},
		Constants: []evaluator.Object{},
		File:      "api.lang",
	}
	workerChunk := &Chunk{
		Code:      []byte{byte(OP_CONST), 0, 1, byte(OP_HALT)},
		Constants: []evaluator.Object{},
		File:      "worker.lang",
	}

	bundle := &Bundle{
		Resources: map[string][]byte{"config.json": []byte(`{"port":8080}`)},
		Commands: map[string]*Bundle{
			"api": {
				MainChunk:     apiChunk,
				Modules:       map[string]*BundledModule{},
				TraitDefaults: map[string]*CompiledFunction{},
				SourceFile:    "api.lang",
			},
			"worker": {
				MainChunk:     workerChunk,
				Modules:       map[string]*BundledModule{},
				TraitDefaults: map[string]*CompiledFunction{},
				SourceFile:    "worker.lang",
			},
		},
	}

	data, err := bundle.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	restored, err := DeserializeAny(data)
	if err != nil {
		t.Fatalf("DeserializeAny failed: %v", err)
	}

	if !restored.IsMultiCommand() {
		t.Fatal("Expected multi-command bundle")
	}
	if len(restored.Commands) != 2 {
		t.Errorf("Commands count: got %d, want 2", len(restored.Commands))
	}

	apiCmd := restored.Commands["api"]
	if apiCmd == nil {
		t.Fatal("Missing 'api' command")
	}
	if apiCmd.MainChunk.File != "api.lang" {
		t.Errorf("api.MainChunk.File: got %q, want %q", apiCmd.MainChunk.File, "api.lang")
	}

	workerCmd := restored.Commands["worker"]
	if workerCmd == nil {
		t.Fatal("Missing 'worker' command")
	}
	if workerCmd.MainChunk.File != "worker.lang" {
		t.Errorf("worker.MainChunk.File: got %q, want %q", workerCmd.MainChunk.File, "worker.lang")
	}

	// Shared resources are on the parent bundle
	if string(restored.Resources["config.json"]) != `{"port":8080}` {
		t.Errorf("Resources not preserved: got %q", restored.Resources["config.json"])
	}
}

func TestBundle_MultiCommand_PackExtractRoundtrip(t *testing.T) {
	host := []byte("fake host binary")
	bundle := &Bundle{
		Resources: map[string][]byte{"static/index.html": []byte("<html>")},
		Commands: map[string]*Bundle{
			"api": {
				MainChunk:     &Chunk{Code: []byte{byte(OP_HALT)}},
				Modules:       map[string]*BundledModule{},
				TraitDefaults: map[string]*CompiledFunction{},
				SourceFile:    "api.lang",
			},
		},
	}

	packed, err := PackSelfContained(host, bundle)
	if err != nil {
		t.Fatalf("PackSelfContained failed: %v", err)
	}

	extracted, err := ExtractEmbeddedBundle(packed)
	if err != nil {
		t.Fatalf("ExtractEmbeddedBundle failed: %v", err)
	}

	if !extracted.IsMultiCommand() {
		t.Error("Expected multi-command after extraction")
	}
	if extracted.Commands["api"] == nil {
		t.Error("Missing 'api' command after extraction")
	}
	if string(extracted.Resources["static/index.html"]) != "<html>" {
		t.Error("Resources not preserved after extraction")
	}
}

func TestBundle_IsMultiCommand(t *testing.T) {
	// Single-command: MainChunk set, no commands
	single := &Bundle{MainChunk: &Chunk{Code: []byte{byte(OP_HALT)}}}
	if single.IsMultiCommand() {
		t.Error("Single-command bundle should not be multi-command")
	}

	// Multi-command: Commands set
	multi := &Bundle{
		Commands: map[string]*Bundle{
			"a": {MainChunk: &Chunk{Code: []byte{byte(OP_HALT)}}},
		},
	}
	if !multi.IsMultiCommand() {
		t.Error("Multi-command bundle should be multi-command")
	}

	// Empty commands map → not multi
	empty := &Bundle{
		MainChunk: &Chunk{Code: []byte{byte(OP_HALT)}},
		Commands:  map[string]*Bundle{},
	}
	if empty.IsMultiCommand() {
		t.Error("Empty Commands map should not be multi-command")
	}
}

func TestBundle_CommandNames(t *testing.T) {
	bundle := &Bundle{
		Commands: map[string]*Bundle{
			"worker": {},
			"api":    {},
			"cron":   {},
		},
	}
	names := bundle.CommandNames()
	expected := []string{"api", "cron", "worker"}
	if len(names) != len(expected) {
		t.Fatalf("CommandNames count: got %d, want %d", len(names), len(expected))
	}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("CommandNames[%d]: got %q, want %q", i, name, expected[i])
		}
	}
}

func TestBundle_ResolveCommand(t *testing.T) {
	sharedResources := map[string][]byte{"shared.txt": []byte("data")}
	bundle := &Bundle{
		Resources: sharedResources,
		Commands: map[string]*Bundle{
			"api": {
				MainChunk:  &Chunk{Code: []byte{byte(OP_HALT)}},
				SourceFile: "api.lang",
			},
		},
	}

	// Resolve existing command
	cmd := bundle.ResolveCommand("api")
	if cmd == nil {
		t.Fatal("Expected to resolve 'api' command")
	}
	if cmd.SourceFile != "api.lang" {
		t.Errorf("Resolved command SourceFile: got %q", cmd.SourceFile)
	}
	// Resources should be inherited
	if string(cmd.Resources["shared.txt"]) != "data" {
		t.Errorf("Resources not inherited: got %q", cmd.Resources["shared.txt"])
	}

	// Resolve non-existing command
	if bundle.ResolveCommand("nonexistent") != nil {
		t.Error("Expected nil for non-existing command")
	}
}

func TestBundle_NilMaps_MultiCommand(t *testing.T) {
	// Multi-command bundle with nil maps → Serialize → DeserializeAny
	bundle := &Bundle{
		Commands: map[string]*Bundle{
			"test": {
				MainChunk:     &Chunk{Code: []byte{byte(OP_HALT)}, Constants: []evaluator.Object{}},
				Modules:       nil,
				TraitDefaults: nil,
			},
		},
	}

	data, err := bundle.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	restored, err := DeserializeAny(data)
	if err != nil {
		t.Fatalf("DeserializeAny failed: %v", err)
	}

	if !restored.IsMultiCommand() {
		t.Error("Expected multi-command after deserialization")
	}
	if restored.Commands == nil {
		t.Error("Commands should not be nil")
	}
}

// --- Spec 1.1: Serialization/Deserialization ---

func TestBundle_SingleCommandRoundtrip(t *testing.T) {
	// 1.1.1 Single-command roundtrip: MainChunk, Modules, TraitDefaults, SourceFile, Resources
	chunk := &Chunk{
		Code:      []byte{byte(OP_HALT)},
		Constants: []evaluator.Object{},
		File:      "main.lang",
	}
	bundle := &Bundle{
		MainChunk: chunk,
		Modules: map[string]*BundledModule{
			"/abs/mod": {Chunk: &Chunk{Code: []byte{byte(OP_HALT)}}, Exports: []string{"x"}},
		},
		TraitDefaults: map[string]*CompiledFunction{},
		SourceFile:    "main.lang",
		Resources:     map[string][]byte{"data.txt": []byte("content")},
	}

	data, err := bundle.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}
	restored, err := DeserializeAny(data)
	if err != nil {
		t.Fatalf("DeserializeAny failed: %v", err)
	}

	if restored.MainChunk == nil {
		t.Error("MainChunk not preserved")
	}
	if len(restored.Modules) != 1 {
		t.Errorf("Modules: got %d, want 1", len(restored.Modules))
	}
	if restored.TraitDefaults == nil {
		t.Error("TraitDefaults not preserved")
	}
	if restored.SourceFile != "main.lang" {
		t.Errorf("SourceFile: got %q", restored.SourceFile)
	}
	if string(restored.Resources["data.txt"]) != "content" {
		t.Errorf("Resources: got %q", restored.Resources["data.txt"])
	}
}

func TestBundle_MultiCommandRoundtrip_SubBundles(t *testing.T) {
	// 1.1.2 Multi-command roundtrip: Commands with 2+ sub-bundles, all restored by name
	bundle := &Bundle{
		Commands: map[string]*Bundle{
			"api": {
				MainChunk:     &Chunk{Code: []byte{byte(OP_HALT)}, File: "api.lang"},
				Modules:       map[string]*BundledModule{},
				TraitDefaults: map[string]*CompiledFunction{},
				SourceFile:    "api.lang",
			},
			"worker": {
				MainChunk:     &Chunk{Code: []byte{byte(OP_HALT)}, File: "worker.lang"},
				Modules:       map[string]*BundledModule{},
				TraitDefaults: map[string]*CompiledFunction{},
				SourceFile:    "worker.lang",
			},
		},
	}

	data, err := bundle.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}
	restored, err := DeserializeAny(data)
	if err != nil {
		t.Fatalf("DeserializeAny failed: %v", err)
	}

	if len(restored.Commands) != 2 {
		t.Fatalf("Commands: got %d, want 2", len(restored.Commands))
	}
	if restored.Commands["api"] == nil || restored.Commands["api"].MainChunk.File != "api.lang" {
		t.Error("api sub-bundle not restored")
	}
	if restored.Commands["worker"] == nil || restored.Commands["worker"].MainChunk.File != "worker.lang" {
		t.Error("worker sub-bundle not restored")
	}
}

func TestBundle_MultiCommandResourcesRoundtrip(t *testing.T) {
	// 1.1.3 Multi-command + Resources: parent has Resources, sub-bundles have none; after roundtrip parent.Resources ok, sub.Resources empty
	bundle := &Bundle{
		Resources: map[string][]byte{"shared.json": []byte(`{}`)},
		Commands: map[string]*Bundle{
			"api": {
				MainChunk:  &Chunk{Code: []byte{byte(OP_HALT)}},
				SourceFile: "api.lang",
				// Resources intentionally nil/empty
			},
		},
	}

	data, err := bundle.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}
	restored, err := DeserializeAny(data)
	if err != nil {
		t.Fatalf("DeserializeAny failed: %v", err)
	}

	if string(restored.Resources["shared.json"]) != `{}` {
		t.Errorf("parent.Resources not preserved: got %q", restored.Resources["shared.json"])
	}
	apiCmd := restored.Commands["api"]
	if apiCmd == nil {
		t.Fatal("api command missing")
	}
	if len(apiCmd.Resources) != 0 {
		t.Errorf("sub-bundle.Resources should be empty, got %d entries", len(apiCmd.Resources))
	}
}

func TestBundle_NilMapsAllInitialized(t *testing.T) {
	// 1.1.4 Nil maps → after DeserializeAny all maps != nil
	chunk := &Chunk{Code: []byte{byte(OP_HALT)}, Constants: []evaluator.Object{}}
	bundle := &Bundle{
		MainChunk:     chunk,
		Modules:       nil,
		TraitDefaults: nil,
		Resources:     nil,
		Commands:      nil,
	}

	data, err := bundle.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}
	restored, err := DeserializeAny(data)
	if err != nil {
		t.Fatalf("DeserializeAny failed: %v", err)
	}

	if restored.Modules == nil {
		t.Error("Modules should be initialized")
	}
	if restored.TraitDefaults == nil {
		t.Error("TraitDefaults should be initialized")
	}
	if restored.Resources == nil {
		t.Error("Resources should be initialized")
	}
	if restored.Commands == nil {
		t.Error("Commands should be initialized")
	}
}

func TestBundle_EmptyCommandsIsSingleCommand(t *testing.T) {
	// 1.1.5 Empty Commands = single-command: MainChunk + Commands = {} → IsMultiCommand() == false
	bundle := &Bundle{
		MainChunk: &Chunk{Code: []byte{byte(OP_HALT)}},
		Commands:  map[string]*Bundle{},
	}

	if bundle.IsMultiCommand() {
		t.Error("Bundle with MainChunk and empty Commands should not be multi-command")
	}
}

func TestDeserializeAny_CorruptedData(t *testing.T) {
	// 1.1.6 Corrupted data: too short, wrong magic, unknown version, garbage gob → all return error
	t.Run("too short", func(t *testing.T) {
		_, err := DeserializeAny([]byte{0x46, 0x58})
		if err == nil {
			t.Error("expected error for too short data")
		}
	})
	t.Run("wrong magic", func(t *testing.T) {
		_, err := DeserializeAny([]byte{0x00, 0x00, 0x00, 0x00, 0x02})
		if err == nil {
			t.Error("expected error for wrong magic")
		}
	})
	t.Run("unknown version", func(t *testing.T) {
		_, err := DeserializeAny([]byte{0x46, 0x58, 0x59, 0x42, 0x99})
		if err == nil {
			t.Error("expected error for unknown version")
		}
	})
	t.Run("garbage gob", func(t *testing.T) {
		data := append([]byte{0x46, 0x58, 0x59, 0x42, 0x02}, []byte("garbage")...)
		_, err := DeserializeAny(data)
		if err == nil {
			t.Error("expected error for garbage gob data")
		}
	})
}

// --- Spec 1.2: Pack/Extract ---

func TestPackExtract_SingleCommand(t *testing.T) {
	// 1.2.7 Single-command pack → extract: MainChunk + Resources
	host := []byte("host")
	bundle := &Bundle{
		MainChunk: &Chunk{Code: []byte{byte(OP_HALT)}, File: "x.lang"},
		Resources: map[string][]byte{"r.txt": []byte("data")},
	}

	packed, err := PackSelfContained(host, bundle)
	if err != nil {
		t.Fatalf("Pack failed: %v", err)
	}
	extracted, err := ExtractEmbeddedBundle(packed)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}
	if extracted == nil {
		t.Fatal("Extract returned nil")
	}
	if extracted.MainChunk == nil {
		t.Error("MainChunk not extracted")
	}
	if string(extracted.Resources["r.txt"]) != "data" {
		t.Errorf("Resources: got %q", extracted.Resources["r.txt"])
	}
}

func TestPackExtract_MultiCommand(t *testing.T) {
	// 1.2.8 Multi-command pack → extract: Commands + parent Resources
	host := []byte("host")
	bundle := &Bundle{
		Resources: map[string][]byte{"config.json": []byte(`{}`)},
		Commands: map[string]*Bundle{
			"api":    {MainChunk: &Chunk{Code: []byte{byte(OP_HALT)}}, SourceFile: "api.lang"},
			"worker": {MainChunk: &Chunk{Code: []byte{byte(OP_HALT)}}, SourceFile: "worker.lang"},
		},
	}

	packed, err := PackSelfContained(host, bundle)
	if err != nil {
		t.Fatalf("Pack failed: %v", err)
	}
	extracted, err := ExtractEmbeddedBundle(packed)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}
	if extracted == nil {
		t.Fatal("Extract returned nil")
	}
	if !extracted.IsMultiCommand() {
		t.Error("Expected multi-command")
	}
	if extracted.Commands["api"] == nil || extracted.Commands["worker"] == nil {
		t.Error("Commands not extracted")
	}
	if string(extracted.Resources["config.json"]) != `{}` {
		t.Error("Parent Resources not extracted")
	}
}

func TestExtractEmbeddedBundle_NoMagicReturnsNilNil(t *testing.T) {
	// 1.2.9 No magic (regular binary) → nil, nil
	// Use string that does NOT end with "FXYS" (last 4 bytes)
	bundle, err := ExtractEmbeddedBundle([]byte("regular binary no embedded bundle"))
	if err != nil {
		t.Errorf("Expected nil error, got: %v", err)
	}
	if bundle != nil {
		t.Error("Expected nil bundle for regular binary")
	}
}

func TestExtractEmbeddedBundle_FooterSizeLargerThanFile(t *testing.T) {
	// 1.2.10 Corrupted footer: bundleSize > fileSize → error
	// Footer: [8-byte size][4-byte FXYS] at end. Size 999999 > available space.
	data := make([]byte, 50)
	binary.LittleEndian.PutUint64(data[38:46], 999999)
	copy(data[46:50], []byte("FXYS"))

	_, err := ExtractEmbeddedBundle(data)
	if err == nil {
		t.Error("Expected error when bundleSize > available space")
	}
}

func TestGetHostBinarySize_Single(t *testing.T) {
	// 1.2.11 GetHostBinarySize — single: Pack, verify == len(host)
	host := []byte("host binary xyz")
	bundle := &Bundle{MainChunk: &Chunk{Code: []byte{byte(OP_HALT)}}}
	packed, err := PackSelfContained(host, bundle)
	if err != nil {
		t.Fatalf("Pack failed: %v", err)
	}
	got := GetHostBinarySize(packed)
	if got != int64(len(host)) {
		t.Errorf("GetHostBinarySize: got %d, want %d", got, len(host))
	}
}

func TestGetHostBinarySize_Multi(t *testing.T) {
	// 1.2.12 GetHostBinarySize — multi-command
	host := []byte("host")
	bundle := &Bundle{
		Commands: map[string]*Bundle{
			"api": {MainChunk: &Chunk{Code: []byte{byte(OP_HALT)}}},
		},
	}
	packed, err := PackSelfContained(host, bundle)
	if err != nil {
		t.Fatalf("Pack failed: %v", err)
	}
	got := GetHostBinarySize(packed)
	if got != int64(len(host)) {
		t.Errorf("GetHostBinarySize (multi): got %d, want %d", got, len(host))
	}
}

func TestGetHostBinarySize_RegularFile(t *testing.T) {
	// 1.2.13 GetHostBinarySize — regular file without bundle → returns len(file)
	data := []byte("no embedded bundle here")
	got := GetHostBinarySize(data)
	if got != int64(len(data)) {
		t.Errorf("GetHostBinarySize on regular file: got %d, want %d", got, len(data))
	}
}

func TestGetHostBinarySize_DoublePack(t *testing.T) {
	// 1.2.14 Double-pack: Pack → pack on top. GetHostBinarySize should return original host size.
	host := []byte("original host")
	bundle1 := &Bundle{MainChunk: &Chunk{Code: []byte{byte(OP_HALT)}}}
	packed1, err := PackSelfContained(host, bundle1)
	if err != nil {
		t.Fatalf("First pack failed: %v", err)
	}

	bundle2 := &Bundle{MainChunk: &Chunk{Code: []byte{byte(OP_HALT)}}}
	packed2, err := PackSelfContained(packed1, bundle2)
	if err != nil {
		t.Fatalf("Second pack failed: %v", err)
	}

	got := GetHostBinarySize(packed2)
	if got != int64(len(host)) {
		t.Errorf("GetHostBinarySize after double-pack: got %d, want %d (original host size)", got, len(host))
	}
}

// --- Spec 1.3: Helper methods ---

func TestBundle_IsMultiCommand_True(t *testing.T) {
	// 1.3.15 IsMultiCommand — true when Commands has 1+ entries
	b := &Bundle{Commands: map[string]*Bundle{"api": {}}}
	if !b.IsMultiCommand() {
		t.Error("IsMultiCommand should be true")
	}
}

func TestBundle_IsMultiCommand_FalseSingle(t *testing.T) {
	// 1.3.16 IsMultiCommand — false when MainChunk set, Commands nil
	b := &Bundle{MainChunk: &Chunk{Code: []byte{byte(OP_HALT)}}}
	if b.IsMultiCommand() {
		t.Error("IsMultiCommand should be false for single-command")
	}
}

func TestBundle_IsMultiCommand_FalseEmptyMap(t *testing.T) {
	// 1.3.17 IsMultiCommand — false when Commands = empty map
	b := &Bundle{MainChunk: &Chunk{Code: []byte{byte(OP_HALT)}}, Commands: map[string]*Bundle{}}
	if b.IsMultiCommand() {
		t.Error("IsMultiCommand should be false for empty Commands map")
	}
}

func TestBundle_CommandNames_Sorted(t *testing.T) {
	// 1.3.18 CommandNames — sorted: worker, api, cron → [api, cron, worker]
	b := &Bundle{Commands: map[string]*Bundle{"worker": {}, "api": {}, "cron": {}}}
	names := b.CommandNames()
	want := []string{"api", "cron", "worker"}
	if len(names) != len(want) {
		t.Fatalf("CommandNames: got %v", names)
	}
	for i, n := range names {
		if n != want[i] {
			t.Errorf("CommandNames[%d]: got %q, want %q", i, n, want[i])
		}
	}
}

func TestBundle_CommandNames_Empty(t *testing.T) {
	// 1.3.19 CommandNames — empty Commands → []
	b := &Bundle{Commands: map[string]*Bundle{}}
	names := b.CommandNames()
	if len(names) != 0 {
		t.Errorf("CommandNames (empty): got %v", names)
	}
}

func TestBundle_CommandNames_Single(t *testing.T) {
	// 1.3.20 CommandNames — one command → [api]
	b := &Bundle{Commands: map[string]*Bundle{"api": {}}}
	names := b.CommandNames()
	if len(names) != 1 || names[0] != "api" {
		t.Errorf("CommandNames (single): got %v", names)
	}
}

func TestBundle_ResolveCommand_Existing(t *testing.T) {
	// 1.3.21 ResolveCommand — existing command returns sub-bundle, Resources inherited
	parent := &Bundle{
		Resources: map[string][]byte{"shared": []byte("data")},
		Commands:  map[string]*Bundle{"api": {MainChunk: &Chunk{Code: []byte{byte(OP_HALT)}}, SourceFile: "api.lang"}},
	}
	cmd := parent.ResolveCommand("api")
	if cmd == nil {
		t.Fatal("ResolveCommand(api) returned nil")
	}
	if cmd.SourceFile != "api.lang" {
		t.Errorf("SourceFile: got %q", cmd.SourceFile)
	}
	if string(cmd.Resources["shared"]) != "data" {
		t.Errorf("Resources not inherited: got %q", cmd.Resources["shared"])
	}
}

func TestBundle_ResolveCommand_Nonexistent(t *testing.T) {
	// 1.3.22 ResolveCommand — nonexistent → nil
	b := &Bundle{Commands: map[string]*Bundle{"api": {}}}
	if b.ResolveCommand("blah") != nil {
		t.Error("ResolveCommand(blah) should return nil")
	}
}

func TestBundle_ResolveCommand_ResourceInheritance(t *testing.T) {
	// 1.3.23 ResolveCommand — parent Resources, sub without → sub.Resources == parent.Resources
	parent := &Bundle{
		Resources: map[string][]byte{"a": []byte("1")},
		Commands:  map[string]*Bundle{"api": {MainChunk: &Chunk{Code: []byte{byte(OP_HALT)}}}},
	}
	cmd := parent.ResolveCommand("api")
	if string(cmd.Resources["a"]) != "1" {
		t.Errorf("Resources not inherited: got %q", cmd.Resources["a"])
	}
}

func TestBundle_ResolveCommand_ResourceMutationIsolation(t *testing.T) {
	// Mutating sub-bundle's inherited Resources must NOT affect parent.
	parent := &Bundle{
		Resources: map[string][]byte{"shared.txt": []byte("parent data")},
		Commands:  map[string]*Bundle{"api": {MainChunk: &Chunk{Code: []byte{byte(OP_HALT)}}}},
	}

	cmd := parent.ResolveCommand("api")

	// Mutate the sub-bundle's resources
	cmd.Resources["shared.txt"] = []byte("mutated")
	cmd.Resources["new.txt"] = []byte("added")

	// Parent must be unaffected
	if string(parent.Resources["shared.txt"]) != "parent data" {
		t.Errorf("parent Resources mutated: got %q, want %q", parent.Resources["shared.txt"], "parent data")
	}
	if _, ok := parent.Resources["new.txt"]; ok {
		t.Error("parent Resources gained new key from sub-bundle mutation")
	}
}

func TestBundle_ResolveCommand_SubOwnResourcesNotOverwritten(t *testing.T) {
	// 1.3.24 ResolveCommand — sub-bundle has own Resources → parent does NOT overwrite
	parent := &Bundle{
		Resources: map[string][]byte{"parent.txt": []byte("parent")},
		Commands: map[string]*Bundle{
			"api": {
				MainChunk: &Chunk{Code: []byte{byte(OP_HALT)}},
				Resources: map[string][]byte{"sub.txt": []byte("sub")},
			},
		},
	}
	cmd := parent.ResolveCommand("api")
	if cmd == nil {
		t.Fatal("ResolveCommand returned nil")
	}
	if string(cmd.Resources["sub.txt"]) != "sub" {
		t.Errorf("Sub Resources overwritten: got %q", cmd.Resources["sub.txt"])
	}
	if _, ok := cmd.Resources["parent.txt"]; ok {
		t.Error("Parent Resources should not overwrite sub's own Resources")
	}
}

func TestBundle_ResolveCommand_ParentNoResources(t *testing.T) {
	// 1.3.25 ResolveCommand — parent without Resources, sub without → no crash, both stay empty
	parent := &Bundle{
		Resources: nil,
		Commands:  map[string]*Bundle{"api": {MainChunk: &Chunk{Code: []byte{byte(OP_HALT)}}}}}
	cmd := parent.ResolveCommand("api")
	if cmd == nil {
		t.Fatal("ResolveCommand returned nil")
	}
	if cmd.Resources == nil {
		// ResolveCommand only sets when len(cmd.Resources)==0 && len(parent.Resources)>0
		// So cmd.Resources can stay nil
		return
	}
	if len(cmd.Resources) != 0 {
		t.Errorf("Sub should have no Resources, got %d", len(cmd.Resources))
	}
}

// --- Spec 1.5: Backward compatibility ---

func TestDeserializeAny_V1LegacyWrapsInBundle(t *testing.T) {
	// 1.5.92 v1 legacy bytecode: DeserializeAny on v1 data → wraps in Bundle, IsMultiCommand == false
	buf := new(bytes.Buffer)
	buf.Write([]byte{0x46, 0x58, 0x59, 0x42}) // FXYB
	buf.WriteByte(bytecodeVersionV1)          // v1
	enc := gob.NewEncoder(buf)
	chunk := Chunk{Code: []byte{byte(OP_HALT)}, Constants: []evaluator.Object{}, File: "legacy.lang"}
	if err := enc.Encode(&chunk); err != nil {
		t.Fatalf("gob encode chunk: %v", err)
	}

	bundle, err := DeserializeAny(buf.Bytes())
	if err != nil {
		t.Fatalf("DeserializeAny v1: %v", err)
	}
	if bundle.MainChunk == nil {
		t.Error("MainChunk should be set from v1 Chunk")
	}
	if bundle.MainChunk.File != "legacy.lang" {
		t.Errorf("MainChunk.File: got %q", bundle.MainChunk.File)
	}
	if bundle.IsMultiCommand() {
		t.Error("v1 legacy should not be multi-command")
	}
}
