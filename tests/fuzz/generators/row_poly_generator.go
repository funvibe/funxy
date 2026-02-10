package generators

import (
	"fmt"
	"strings"
)

// RowPolyGenerator generates code specifically for testing row polymorphism.
type RowPolyGenerator struct {
	*Generator
}

func NewRowPolyGenerator(data []byte) *RowPolyGenerator {
	return &RowPolyGenerator{
		Generator: NewFromData(data),
	}
}

// GenerateChainedRecordOps generates a list of records processed by a chain of maps.
// This tests the "Greedy Inference" and "Mutual Row Extension" logic.
func (g *RowPolyGenerator) GenerateChainedRecordOps() string {
	// 1. Create a list of records with varying fields
	fieldPool := []string{"a", "b", "c", "d", "e", "f"}

	var sb strings.Builder
	sb.WriteString("import \"lib/list\" (map)\n\n")
	sb.WriteString("fun main() {\n")
	sb.WriteString("  data = [\n")

	count := g.src.Intn(5) + 2
	for i := 0; i < count; i++ {
		sb.WriteString("    { ")
		// Add random fields from pool
		fieldCount := g.src.Intn(4) + 1
		for j := 0; j < fieldCount; j++ {
			if j > 0 {
				sb.WriteString(", ")
			}
			f := fieldPool[g.src.Intn(len(fieldPool))]
			val := g.src.Intn(100)
			sb.WriteString(fmt.Sprintf("%s: %d", f, val))
		}
		sb.WriteString(" },\n")
	}
	sb.WriteString("  ]\n\n")

	sb.WriteString("  res = data\n")

	// 2. Generate chain of maps accessing different fields
	chainLen := g.src.Intn(5) + 1
	for i := 0; i < chainLen; i++ {
		targetField := fieldPool[g.src.Intn(len(fieldPool))]
		sb.WriteString(fmt.Sprintf("    |> map(\\x -> { val = x.%s; x })\n", targetField))
	}

	sb.WriteString("  print(len(res))\n")
	sb.WriteString("}")

	return sb.String()
}

// GenerateRecursiveRecord generates a recursive record structure and a function to traverse it.
// This tests for infinite loops during unification.
func (g *RowPolyGenerator) GenerateRecursiveRecord() string {
	return `
type alias Node = { val: Int, next: Option<Node> }

fun traverse(node) {
    match node.next {
        Some(n) -> node.val + traverse(n)
        None -> node.val
    }
}

fun main() {
    n1 = { val: 1, next: None }
    n2 = { val: 2, next: Some(n1) }
    print(traverse(n2))
}
`
}

// GenerateConflictingRecords generates a list of records with conflicting types for the same field.
// This should trigger a type error (or proper handling if union types are inferred).
func (g *RowPolyGenerator) GenerateConflictingRecords() string {
	return `
fun main() {
    list = [
        { a: 1, b: "string" },
        { a: 2, b: 100 }
    ]
    // Consuming function requiring unified type
    fun consume(l) {
        l[0].a
    }
    print(consume(list))
}
`
}
