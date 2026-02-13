package vm

import (
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
