package generators

import (
	"fmt"
	"strings"
)

// StdLibGenerator generates code targeting standard library functions.
type StdLibGenerator struct {
	*Generator
}

func NewStdLibGenerator(seed int64) *StdLibGenerator {
	return &StdLibGenerator{
		Generator: New(seed),
	}
}

func (g *StdLibGenerator) GenerateStdLibProgram() string {
	var sb strings.Builder

	// Generate imports if needed (assuming stdlib is built-in or imported)
	// sb.WriteString("import \"std/list\"\n")

	count := g.src.Intn(5) + 1
	for i := 0; i < count; i++ {
		sb.WriteString(g.GenerateStdLibCall())
		sb.WriteString("\n")
	}
	return sb.String()
}

func (g *StdLibGenerator) GenerateStdLibCall() string {
	choice := g.src.Intn(3)
	switch choice {
	case 0:
		return g.GenerateListOp()
	case 1:
		return g.GenerateMapOp()
	case 2:
		return g.GenerateStringOp()
	default:
		return g.GenerateListOp()
	}
}

func (g *StdLibGenerator) GenerateListOp() string {
	ops := []string{
		"List.map", "List.filter", "List.fold", "List.reverse", "List.append",
		"List.head", "List.tail", "List.length",
	}
	op := ops[g.src.Intn(len(ops))]

	list := g.GenerateListLiteral()

	switch op {
	case "List.map":
		// List.map(list, fn)
		return fmt.Sprintf("%s(%s, fun(x) { x })", op, list)
	case "List.filter":
		return fmt.Sprintf("%s(%s, fun(x) { true })", op, list)
	case "List.fold":
		return fmt.Sprintf("%s(%s, 0, fun(acc, x) { acc })", op, list)
	case "List.append":
		return fmt.Sprintf("%s(%s, %s)", op, list, g.GenerateListLiteral())
	default:
		return fmt.Sprintf("%s(%s)", op, list)
	}
}

func (g *StdLibGenerator) GenerateMapOp() string {
	ops := []string{
		"Map.get", "Map.put", "Map.keys", "Map.values", "Map.merge",
	}
	op := ops[g.src.Intn(len(ops))]

	m := g.GenerateMapLiteral()

	switch op {
	case "Map.get":
		return fmt.Sprintf("%s(%s, %s)", op, m, g.GenerateExpression())
	case "Map.put":
		return fmt.Sprintf("%s(%s, %s, %s)", op, m, g.GenerateExpression(), g.GenerateExpression())
	case "Map.merge":
		return fmt.Sprintf("%s(%s, %s)", op, m, g.GenerateMapLiteral())
	default:
		return fmt.Sprintf("%s(%s)", op, m)
	}
}

func (g *StdLibGenerator) GenerateStringOp() string {
	ops := []string{
		"String.length", "String.concat", "String.split", "String.trim",
	}
	op := ops[g.src.Intn(len(ops))]

	s := g.GenerateLiteral() // Assuming it generates strings sometimes

	switch op {
	case "String.concat":
		return fmt.Sprintf("%s(%s, %s)", op, s, g.GenerateLiteral())
	case "String.split":
		return fmt.Sprintf("%s(%s, \" \")", op, s)
	default:
		return fmt.Sprintf("%s(%s)", op, s)
	}
}
