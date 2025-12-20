Add bytecode compilation and execution support

Implements Phase 1 of making FunXY a compiling language:

- Added `-c` flag to compile source to .fbc bytecode files
- Added `-r` flag to run pre-compiled bytecode
- Implemented serialization/deserialization for Chunk with gob encoding
- Added custom GobEncode/GobDecode for List and OperatorFunction
- Modified compiler to preserve pending imports in bytecode
- Updated handleRunCompiled to process imports before execution
- Updated help documentation with new commands and limitations

Key technical details:
- Bytecode format: FXYB magic + version + gob-encoded Chunk
- Imports are serialized as PendingImport structs
- OperatorFunction only serializes operator string (not Evaluator)
- List serializes as simple slice to avoid PersistentVector complexity

All existing tests pass. Single-file programs with imports work correctly.
Module packages require tree-walk mode (documented limitation).

Files changed:
- cmd/funxy/main.go: Added compile/run handlers
- internal/vm/chunk.go: Added serialization + PendingImports field
- internal/vm/compiler.go: Copy imports to chunk
- internal/evaluator/object.go: Custom serialization methods
- internal/modules/docs.go: Updated help text
