// symbols/symbol_table.go - Main symbol table entry point
//
// This file has been split into focused modules for maintainability:
// - symbol_table_core.go: Core types, Symbol struct, basic utilities
// - symbol_table_init.go: Prelude initialization and built-in types
// - symbol_table_operations.go: Basic symbol table operations (define, find, etc.)
// - symbol_table_aliases.go: Type alias handling and resolution
// - symbol_table_traits.go: Trait definitions and methods
// - symbol_table_implementations.go: Trait implementations and instance methods
// - symbol_table_resolution.go: Symbol resolution, extension methods, type parameters
// - symbol_table_advanced.go: Advanced features and SymbolTable struct definition

package symbols

// All symbol table functionality is now distributed across the focused modules above.
// This file serves as the main entry point and maintains the package structure.
