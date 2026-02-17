package generators

import (
	"fmt"
	"math/rand"
	"strings"
)

// RandomSource abstracts the source of randomness.
type RandomSource interface {
	Intn(n int) int
	Float64() float64
}

// RandSource wraps math/rand.
type RandSource struct {
	*rand.Rand
}

// ByteSource uses a byte slice as a source of randomness.
type ByteSource struct {
	data []byte
	pos  int
}

func (s *ByteSource) Intn(n int) int {
	if n <= 0 {
		return 0
	}
	if s.pos >= len(s.data) {
		return 0
	}
	v := int(s.data[s.pos])
	s.pos++
	return v % n
}

func (s *ByteSource) Float64() float64 {
	if s.pos >= len(s.data) {
		return 0.0
	}
	v := int(s.data[s.pos])
	s.pos++
	return float64(v) / 255.0
}

// Generator generates random Funxy code.
type Generator struct {
	src   RandomSource
	depth int
	vars  []string
}

const (
	MaxDepth      = 5
	MaxStatements = 5
)

func New(seed int64) *Generator {
	return &Generator{
		src:  &RandSource{rand.New(rand.NewSource(seed))},
		vars: []string{"x", "y", "z", "a", "b"},
	}
}

func NewFromData(data []byte) *Generator {
	return &Generator{
		src:  &ByteSource{data: data},
		vars: []string{"x", "y", "z", "a", "b"},
	}
}

// Intn exposes the random source's Intn method for embedded structs.
func (g *Generator) Intn(n int) int {
	return g.src.Intn(n)
}

// Src returns the random source of the generator.
func (g *Generator) Src() RandomSource {
	return g.src
}

func (g *Generator) GenerateProgram() string {
	var sb strings.Builder
	count := g.src.Intn(5) + 1
	for i := 0; i < count; i++ {
		sb.WriteString(g.GenerateTopLevelStatement())
		sb.WriteString("\n")
		sb.WriteString(g.GenerateNoise())
	}
	// Append nil to ensure consistent return value (avoiding Nil vs Closure discrepancies on declarations)
	sb.WriteString("nil")
	return sb.String()
}

func (g *Generator) GenerateNoise() string {
	// 10% chance to generate noise
	if g.src.Intn(10) != 0 {
		return ""
	}

	var sb strings.Builder
	count := g.src.Intn(3) + 1
	for i := 0; i < count; i++ {
		switch g.src.Intn(3) {
		case 0:
			sb.WriteString(" ")
		case 1:
			sb.WriteString("\t")
		case 2:
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// MaybeNewline returns "\n" with ~30% probability, otherwise " ".
// Use this at syntactic boundaries where newlines should be legal
// (after ->, after commas in lists, after {, before }, etc.)
func (g *Generator) MaybeNewline() string {
	if g.src.Intn(3) == 0 {
		return "\n"
	}
	return " "
}

func (g *Generator) GenerateTopLevelStatement() string {
	if g.depth > MaxDepth {
		return "print(\"limit\")"
	}
	g.depth++
	defer func() { g.depth-- }()

	// Weighted choice
	choice := g.src.Intn(15)
	switch {
	case choice < 3: // 0, 1, 2
		return g.GenerateVarDecl()
	case choice < 4: // 3
		return g.GenerateConstDecl()
	case choice < 6: // 4, 5
		return g.GenerateFunctionDecl()
	case choice < 8: // 6, 7
		return g.GenerateIfExpression()
	case choice < 9: // 8
		return g.GenerateLoop()
	case choice < 10: // 9
		return g.GenerateMatchExpression()
	case choice < 11: // 10
		return g.GenerateTypeDecl()
	case choice < 12: // 11
		return g.GenerateTraitDecl()
	case choice < 13: // 12
		return g.GenerateInstanceDecl()
	default: // 13, 14
		return g.GenerateExpression()
	}
}

func (g *Generator) GenerateBlockStatement() string {
	if g.depth > MaxDepth {
		return "print(\"limit\")"
	}
	g.depth++
	defer func() { g.depth-- }()

	// Weighted choice - Exclude Type, Trait, Instance
	choice := g.src.Intn(13)
	switch {
	case choice < 3: // 0, 1, 2
		return g.GenerateVarDecl()
	case choice < 4: // 3
		return g.GenerateConstDecl()
	case choice < 6: // 4, 5
		return g.GenerateFunctionDecl()
	case choice < 8: // 6, 7
		return g.GenerateIfExpression()
	case choice < 9: // 8
		return g.GenerateLoop()
	case choice < 10: // 9
		return g.GenerateMatchExpression()
	default: // 10, 11, 12
		return g.GenerateExpression()
	}
}

// GenerateStatement is kept for compatibility if needed, but delegates to BlockStatement
// as that is the safer default for recursive calls unless specified.
func (g *Generator) GenerateStatement() string {
	return g.GenerateBlockStatement()
}

func (g *Generator) GenerateVarDecl() string {
	name := g.GenerateIdentifier()
	// Optionally add type annotation
	if g.src.Intn(2) == 0 {
		return fmt.Sprintf("%s : %s = %s", name, g.GenerateType(), g.GenerateExpression())
	}
	return fmt.Sprintf("%s = %s", name, g.GenerateExpression())
}

func (g *Generator) GenerateConstDecl() string {
	name := "k" + g.GenerateIdentifier()
	expr := g.GenerateExpression()

	// Mix syntax styles
	switch g.src.Intn(3) {
	case 0:
		// const name = value
		return fmt.Sprintf("const %s = %s", name, expr)
	case 1:
		// name :- value
		return fmt.Sprintf("%s :- %s", name, expr)
	default:
		// const name : Type = value (or name : Type :- value)
		typ := g.GenerateType()
		if g.src.Intn(2) == 0 {
			return fmt.Sprintf("const %s : %s = %s", name, typ, expr)
		}
		return fmt.Sprintf("%s : %s :- %s", name, typ, expr)
	}
}

func (g *Generator) GenerateFunctionDecl() string {
	name := "fn_" + g.GenerateIdentifier()

	// Generic parameters (optional)
	genParamsStr := ""
	var availableGens []string

	if g.src.Intn(3) == 0 { // 33% chance
		genCount := g.src.Intn(3) + 1 // 1 to 3 generic params
		for i := 0; i < genCount; i++ {
			// Use single letters for generics to look more natural: a, b, c, m, f
			genName := []string{"a", "b", "c", "d", "e", "m", "f"}[g.src.Intn(7)]
			// Avoid duplicates
			isDup := false
			for _, existing := range availableGens {
				if existing == genName {
					isDup = true
					break
				}
			}
			if !isDup {
				availableGens = append(availableGens, genName)
			}
		}
		if len(availableGens) > 0 {
			genParamsStr = fmt.Sprintf("<%s>", strings.Join(availableGens, ", "))
		}
	}

	// Params
	paramCount := g.src.Intn(3)
	var params []string
	for i := 0; i < paramCount; i++ {
		pName := fmt.Sprintf("p%d", i)

		// If we have generics, try to use them in types
		var pType string
		if len(availableGens) > 0 && g.src.Intn(2) == 0 {
			pType = g.GenerateTypeWithGenerics(availableGens)
		} else {
			pType = g.GenerateType()
		}

		// Higher-Rank Types (forall) - 10% chance if no generics used yet
		if len(availableGens) == 0 && g.src.Intn(10) == 0 {
			pType = fmt.Sprintf("(forall a. a -> a)")
		}

		params = append(params, fmt.Sprintf("%s: %s", pName, pType))
	}

	// Return type (optional)
	retType := ""
	if g.src.Intn(2) == 0 {
		var rType string
		if len(availableGens) > 0 && g.src.Intn(2) == 0 {
			rType = g.GenerateTypeWithGenerics(availableGens)
		} else {
			rType = g.GenerateType()
		}
		retType = " -> " + rType
	}

	body := g.GenerateBlock()
	return fmt.Sprintf("fun %s%s(%s)%s %s", name, genParamsStr, strings.Join(params, ", "), retType, body)
}

// GenerateTypeWithGenerics creates types using the provided generic parameters
func (g *Generator) GenerateTypeWithGenerics(gens []string) string {
	if len(gens) == 0 {
		return g.GenerateType()
	}

	// Pick a random generic
	gen := gens[g.src.Intn(len(gens))]

	switch g.src.Intn(6) {
	case 0:
		return gen // Just the generic 'a'
	case 1:
		return fmt.Sprintf("List<%s>", gen) // List<a>
	case 2:
		// Function type: a -> b (if b exists)
		other := gens[g.src.Intn(len(gens))]
		return fmt.Sprintf("%s -> %s", gen, other)
	case 3:
		// Complex: Map<String, a>
		return fmt.Sprintf("Map<String, %s>", gen)
	case 4:
		// Higher-kinded usage if name suggests it (m, f)
		if gen == "m" || gen == "f" {
			other := gens[g.src.Intn(len(gens))]
			return fmt.Sprintf("%s<%s>", gen, other) // m<a>
		}
		return gen
	default:
		return fmt.Sprintf("Option<%s>", gen)
	}
}

func (g *Generator) GenerateBlock() string {
	var sb strings.Builder
	sb.WriteString("{" + g.MaybeNewline())
	count := g.src.Intn(3) + 1
	for i := 0; i < count; i++ {
		sb.WriteString(g.GenerateBlockStatement())
		sb.WriteString("\n")
		sb.WriteString(g.GenerateNoise())
	}
	sb.WriteString(g.MaybeNewline() + "}")
	return sb.String()
}

func (g *Generator) GenerateLoop() string {
	body := g.GenerateBlock()
	// 50% for-in, 50% for-condition
	if g.src.Intn(2) == 0 {
		iter := g.GenerateIdentifier()
		list := g.GenerateExpression()
		return fmt.Sprintf("for %s in %s%s%s", iter, list, g.MaybeNewline(), body)
	}
	cond := g.GenerateExpression()
	return fmt.Sprintf("for %s%s%s", cond, g.MaybeNewline(), body)
}

func (g *Generator) GenerateIfExpression() string {
	cond := g.GenerateBooleanExpression()
	cons := g.GenerateBlock()
	alt := g.GenerateBlock()
	return fmt.Sprintf("if %s %s%selse %s", cond, cons, g.MaybeNewline(), alt)
}

func (g *Generator) GenerateMatchExpression() string {
	expr := g.GenerateExpression()
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("match %s {\n", expr))

	armCount := g.src.Intn(3) + 1
	for i := 0; i < armCount; i++ {
		sb.WriteString(g.GenerateMatchArm())
		sb.WriteString("\n")
	}
	// Always add a wildcard arm to ensure exhaustiveness
	sb.WriteString("_ ->" + g.MaybeNewline() + g.GenerateExpression() + "\n")
	sb.WriteString("}")
	return sb.String()
}

func (g *Generator) GenerateMatchArm() string {
	pattern := g.GeneratePattern()

	// 25% chance to add a guard
	guard := ""
	if g.src.Intn(4) == 0 {
		guard = " if " + g.GenerateBooleanExpression()
	}

	expr := g.GenerateExpression()
	// Randomly put newline after -> to stress-test parser
	return fmt.Sprintf("%s%s ->%s(%s)", pattern, guard, g.MaybeNewline(), expr)
}

func (g *Generator) GeneratePattern() string {
	switch g.src.Intn(4) {
	case 0:
		return g.GeneratePatternLiteral()
	case 1:
		return "_"
	case 2:
		return g.GenerateIdentifier()
	case 3:
		return fmt.Sprintf("(%s, %s)", g.GeneratePattern(), g.GeneratePattern())
	default:
		return g.GeneratePatternLiteral()
	}
}

func (g *Generator) GeneratePatternLiteral() string {
	switch g.src.Intn(3) {
	case 0:
		return fmt.Sprintf("%d", g.src.Intn(100))
	case 1:
		return fmt.Sprintf("\"str_%d\"", g.src.Intn(100))
	default:
		return []string{"true", "false"}[g.src.Intn(2)]
	}
}

func (g *Generator) GenerateTypeDecl() string {
	name := "Type" + g.GenerateIdentifier()

	// Generic parameters (optional)
	genParams := ""
	if g.src.Intn(3) == 0 { // 33% chance
		genCount := g.src.Intn(2) + 1 // 1 or 2 generic params
		var gens []string
		for i := 0; i < genCount; i++ {
			gens = append(gens, fmt.Sprintf("%s", g.GenerateIdentifier()))
		}
		genParams = fmt.Sprintf("<%s>", strings.Join(gens, ", "))
	}

	// Kind annotation (optional)
	kindAnnotation := ""
	if g.src.Intn(4) == 0 { // 25% chance
		kinds := []string{"*", "* -> *", "* -> * -> *", "(* -> *) -> *"}
		kindAnnotation = fmt.Sprintf(" : %s", kinds[g.src.Intn(len(kinds))])
	}

	// Simple alias, record, or generic alias
	choice := g.src.Intn(3)
	switch choice {
	case 0:
		return fmt.Sprintf("type alias %s%s%s = %s", name, genParams, kindAnnotation, g.GenerateType())
	case 1:
		return fmt.Sprintf("type alias %s%s%s = { f1: %s, f2: %s }", name, genParams, kindAnnotation, g.GenerateType(), g.GenerateType())
	default:
		// Generic alias like type alias MyList<T> = List<T>
		if genParams != "" {
			return fmt.Sprintf("type alias %s%s%s = List<%s>", name, genParams, kindAnnotation, strings.Split(genParams, "<")[1][:1])
		}
		return fmt.Sprintf("type alias %s%s%s = %s", name, genParams, kindAnnotation, g.GenerateType())
	}
}

func (g *Generator) GenerateTraitDecl() string {
	name := "Trait" + g.GenerateIdentifier()

	// 1. Functional Dependencies (FunDeps) - 20% chance
	if g.src.Intn(5) == 0 {
		return fmt.Sprintf("trait %s<c, e> | c -> e { fun item(coll: c) -> e }", name)
	}

	// 2. Kind Annotations on Trait Parameters - 20% chance
	if g.src.Intn(5) == 0 {
		return fmt.Sprintf("trait %s<m : * -> *> { fun pure<a>(x: a) -> m<a> }", name)
	}

	// Generate complex constraints
	constraints := g.GenerateTraitConstraints()

	// Simple trait or trait with constraints
	if g.src.Intn(2) == 0 {
		// Simple trait without constraints
		return fmt.Sprintf("trait %s<t> { fun method(x: t) -> t }", name)
	}

	// Trait with constraints
	return fmt.Sprintf("trait %s<%s> { fun method(x: t) -> t }", name, constraints)
}

func (g *Generator) GenerateTraitConstraints() string {
	// Generate complex trait constraints
	choice := g.src.Intn(6)
	switch choice {
	case 0:
		// Single constraint
		return "t: Show"
	case 1:
		// Multiple constraints with commas
		traits := []string{"Show", "Eq", "Ord", "Hash"}
		count := g.src.Intn(2) + 2 // 2 or 3 traits
		var selected []string
		for i := 0; i < count; i++ {
			selected = append(selected, traits[g.src.Intn(len(traits))])
		}
		return fmt.Sprintf("t: %s", strings.Join(selected, ", "))
	case 2:
		// Higher-kinded constraint: F: Functor
		return "F: Functor"
	case 3:
		// Complex constraint with kind annotation: M: (* -> *) -> *"
		return "M: (* -> *) -> *"
	case 4:
		// Nested constraints: t: Bar<Baz<c>>
		return "t: Iterator<Item<String>>"
	case 5:
		// Multiple params with constraints
		return "a: Show, b: Eq"
	default:
		return "t: Show"
	}
}

func (g *Generator) GenerateInstanceDecl() string {
	// We need existing traits and types, but for fuzzing we can try to generate valid-looking ones
	// or rely on built-ins.
	// Let's use a built-in trait for a generated type if possible, or just generate syntax.
	return fmt.Sprintf("instance Show Int { fun show(x: Int) -> String { \"int\" } }")
}

func (g *Generator) GenerateExpression() string {
	if g.depth > MaxDepth {
		return g.GenerateLiteral()
	}
	g.depth++
	defer func() { g.depth-- }()

	switch g.src.Intn(18) {
	case 0, 1, 2:
		return g.GenerateLiteral()
	case 3:
		return g.GenerateIdentifier()
	case 4:
		return g.GenerateBinaryExpression()
	case 5:
		return g.GenerateUnaryExpression()
	case 6:
		return g.GenerateCallExpression()
	case 7:
		return g.GenerateListLiteral()
	case 8:
		return g.GenerateRecordLiteral()
	case 9:
		return g.GenerateTupleLiteral()
	case 10:
		return g.GenerateListComprehension()
	case 11:
		return g.GenerateInterpolatedString()
	case 12:
		return g.GenerateBitSyntax()
	case 13:
		return g.GenerateMapLiteral()
	case 14:
		return g.GenerateBytesLiteral()
	case 15:
		return g.GenerateCharLiteral()
	case 16:
		return g.GenerateLambda()
	case 17:
		return g.GeneratePipeExpression()
	default:
		return g.GenerateLiteral()
	}
}

func (g *Generator) GenerateLambda() string {
	paramCount := g.src.Intn(3) + 1
	var params []string
	for i := 0; i < paramCount; i++ {
		params = append(params, fmt.Sprintf("p%d", i))
	}
	body := g.GenerateExpression()
	return fmt.Sprintf("(\\%s ->%s%s)", strings.Join(params, ", "), g.MaybeNewline(), body)
}

func (g *Generator) GeneratePipeExpression() string {
	left := g.GenerateExpression()
	right := g.GenerateIdentifier()
	op := []string{"|>", "|>>"}[g.src.Intn(2)]
	return fmt.Sprintf("(%s%s%s %s)", left, g.MaybeNewline(), op, right)
}

func (g *Generator) GenerateBinaryExpression() string {
	op := []string{"+", "-", "*", "/", "==", "!=", "<", ">", "&&", "||", "|>", "|>>"}[g.src.Intn(12)]
	return fmt.Sprintf("(%s %s %s)", g.GenerateExpression(), op, g.GenerateExpression())
}

func (g *Generator) GenerateUnaryExpression() string {
	op := []string{"-", "!"}[g.src.Intn(2)]
	return fmt.Sprintf("(%s %s)", op, g.GenerateExpression())
}

func (g *Generator) GenerateCallExpression() string {
	// Call a builtin or generated function
	funcs := []string{"print", "len", "fn_x", "fn_y"}
	name := funcs[g.src.Intn(len(funcs))]

	argCount := g.src.Intn(3)
	var args []string
	for i := 0; i < argCount; i++ {
		args = append(args, g.GenerateExpression())
	}
	// Use newlines between args sometimes
	sep := "," + g.MaybeNewline()
	return fmt.Sprintf("%s(%s)", name, strings.Join(args, sep))
}

func (g *Generator) GenerateListLiteral() string {
	count := g.src.Intn(4)
	var elements []string
	for i := 0; i < count; i++ {
		elements = append(elements, g.GenerateExpression())
	}
	sep := "," + g.MaybeNewline()
	return fmt.Sprintf("[%s]", strings.Join(elements, sep))
}

func (g *Generator) GenerateRecordLiteral() string {
	var sb strings.Builder
	sb.WriteString("{")

	// Optional spread - only use record expressions for spread
	if g.src.Intn(4) == 0 {
		// Generate a valid record for spread
		sb.WriteString(fmt.Sprintf("...%s", g.GenerateValidRecordForSpread()))
		if g.src.Intn(2) == 0 {
			sb.WriteString(", ")
		}
	}

	count := g.src.Intn(3)
	for i := 0; i < count; i++ {
		if i > 0 || (sb.Len() > 1 && !strings.HasSuffix(sb.String(), ", ")) {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("f%d: %s", i, g.GenerateExpression()))
	}
	sb.WriteString("}")
	return sb.String()
}

// GenerateValidRecordForSpread creates a simple record that can be used in spread
func (g *Generator) GenerateValidRecordForSpread() string {
	// Create a simple record with 1-2 fields
	count := g.src.Intn(2) + 1
	var fields []string
	for i := 0; i < count; i++ {
		// Use simple literals to avoid complex escaping issues
		fields = append(fields, fmt.Sprintf("sf%d: %d", i, g.src.Intn(100)))
	}
	return fmt.Sprintf("{ %s }", strings.Join(fields, ", "))
}

// GenerateBooleanExpression creates a boolean expression for conditions
func (g *Generator) GenerateBooleanExpression() string {
	if g.depth > MaxDepth {
		return []string{"true", "false"}[g.src.Intn(2)]
	}
	g.depth++
	defer func() { g.depth-- }()

	choice := g.src.Intn(3)
	switch choice {
	case 0:
		// Simple boolean literal
		return []string{"true", "false"}[g.src.Intn(2)]
	case 1:
		// Comparison expression
		return fmt.Sprintf("%s == %s", g.GenerateLiteral(), g.GenerateLiteral())
	case 2:
		// Logical expression
		return fmt.Sprintf("%s && %s", g.GenerateBooleanExpression(), g.GenerateBooleanExpression())
	default:
		return "true"
	}
}

func (g *Generator) GenerateMapLiteral() string {
	var sb strings.Builder
	sb.WriteString("%{")
	count := g.src.Intn(4)
	for i := 0; i < count; i++ {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%s => %s", g.GenerateExpression(), g.GenerateExpression()))
	}
	sb.WriteString("}")
	return sb.String()
}

func (g *Generator) GenerateTupleLiteral() string {
	count := g.src.Intn(3) + 2
	var elements []string
	for i := 0; i < count; i++ {
		elements = append(elements, g.GenerateExpression())
	}
	return fmt.Sprintf("(%s)", strings.Join(elements, ", "))
}

func (g *Generator) GenerateListComprehension() string {
	// [ output | pattern <- iterable, condition ]
	output := g.GenerateExpression()

	// Generator clause
	pattern := g.GenerateIdentifier() // Keep it simple for now
	iterable := g.GenerateListLiteral()

	// Optional filter
	filter := ""
	if g.src.Intn(2) == 0 {
		filter = fmt.Sprintf(", %s > 0", pattern) // Simple condition
	}

	return fmt.Sprintf("[%s | %s <- %s%s]", output, pattern, iterable, filter)
}

func (g *Generator) GenerateInterpolatedString() string {
	return fmt.Sprintf("\"val: ${%s}\"", g.GenerateExpression())
}

func (g *Generator) GenerateBitSyntax() string {
	switch g.src.Intn(3) {
	case 0:
		return fmt.Sprintf("#b\"%b\"", g.src.Intn(16))
	case 1:
		return fmt.Sprintf("#x\"%x\"", g.src.Intn(255))
	default:
		return fmt.Sprintf("#o\"%o\"", g.src.Intn(64))
	}
}

func (g *Generator) GenerateBytesLiteral() string {
	switch g.src.Intn(3) {
	case 0:
		return fmt.Sprintf("@\"str_%d\"", g.src.Intn(100))
	case 1:
		return fmt.Sprintf("@x\"%x\"", g.src.Intn(1000))
	default:
		return fmt.Sprintf("@b\"%b\"", g.src.Intn(255))
	}
}

func (g *Generator) GenerateCharLiteral() string {
	chars := []string{"'a'", "'z'", "'0'", "'\\n'", "'\\t'", "'\\''", "'\\u00FF'"}
	return chars[g.src.Intn(len(chars))]
}

func (g *Generator) GenerateLiteral() string {
	switch g.src.Intn(8) {
	case 0:
		return fmt.Sprintf("%d", g.src.Intn(100))
	case 1:
		// Enhanced string generation
		if g.src.Intn(5) == 0 {
			return "\"\\n\\t\\\"\"" // Escaped chars
		} else if g.src.Intn(5) == 0 {
			return "\"👋 🌍\"" // Unicode
		}
		return fmt.Sprintf("\"str_%d\"", g.src.Intn(100))
	case 2:
		return []string{"true", "false"}[g.src.Intn(2)]
	case 3:
		return fmt.Sprintf("%d.0", g.src.Intn(100))
	case 4:
		return fmt.Sprintf("%dn", g.src.Intn(100)) // BigInt
	case 5:
		return g.GenerateCharLiteral()
	case 6:
		return g.GenerateBytesLiteral()
	default:
		return "nil"
	}
}

func (g *Generator) GenerateIdentifier() string {
	if len(g.vars) > 0 && g.src.Intn(2) == 0 {
		return g.vars[g.src.Intn(len(g.vars))]
	}
	// Generate new name
	return []string{"x", "y", "z", "a", "b", "foo", "bar"}[g.src.Intn(7)]
}

func (g *Generator) GenerateType() string {
	basicTypes := []string{"Int", "String", "Bool", "Float", "Any", "BigInt"}
	if g.depth > MaxDepth || g.src.Intn(3) < 2 {
		return basicTypes[g.src.Intn(len(basicTypes))]
	}
	g.depth++
	defer func() { g.depth-- }()

	// Complex types
	switch g.src.Intn(8) {
	case 0:
		return fmt.Sprintf("List<%s>", g.GenerateType())
	case 1:
		return fmt.Sprintf("Map<%s, %s>", g.GenerateType(), g.GenerateType())
	case 2:
		return fmt.Sprintf("Option<%s>", g.GenerateType())
	case 3:
		return fmt.Sprintf("Result<%s, %s>", g.GenerateType(), g.GenerateType())
	case 4:
		// Nested generics: List<Option<Result<Error, String>>>
		return fmt.Sprintf("List<Option<Result<%s, %s>>>", g.GenerateType(), g.GenerateType())
	case 5:
		// Higher-kinded type: Functor<F>
		return fmt.Sprintf("Functor<%s>", g.GenerateIdentifier())
	case 6:
		// Complex nested: Map<String, List<Option<Int>>>
		return fmt.Sprintf("Map<String, List<Option<%s>>>", g.GenerateType())
	case 7:
		// Tuple type
		return fmt.Sprintf("(%s, %s)", g.GenerateType(), g.GenerateType())
	default:
		return "Int"
	}
}
