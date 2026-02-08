package generators

import (
	"fmt"
	"strings"
)

// AsyncGenerator generates code with heavy async/await usage.
type AsyncGenerator struct {
	*Generator
}

func NewAsyncGenerator(data []byte) *AsyncGenerator {
	return &AsyncGenerator{
		Generator: NewFromData(data),
	}
}

// GenerateAsyncProgram creates a program that spawns many tasks and awaits them.
func (g *AsyncGenerator) GenerateAsyncProgram() string {
	var sb strings.Builder
	sb.WriteString("import \"lib/task\" (*)\n")
	sb.WriteString("import \"lib/list\" (map, range)\n\n")
	sb.WriteString("fun main() {\n")

	// Generate a mix of patterns
	count := g.Src().Intn(5) + 1
	for i := 0; i < count; i++ {
		sb.WriteString(g.GenerateAsyncPattern())
		sb.WriteString("\n")
	}

	sb.WriteString("  print(\"done\")\n")
	sb.WriteString("}\n")
	return sb.String()
}

func (g *AsyncGenerator) GenerateAsyncPattern() string {
	choice := g.Src().Intn(5)
	switch choice {
	case 0:
		return g.GenerateTaskSpawn()
	case 1:
		return g.GenerateTaskChain()
	case 2:
		return g.GenerateParallelMap()
	case 3:
		return g.GenerateRecursiveAsync()
	case 4:
		return g.GenerateRaceSimulation()
	default:
		return g.GenerateTaskSpawn()
	}
}

func (g *AsyncGenerator) GenerateTaskSpawn() string {
	// t = async(fun() { ... })
	// await(t)
	body := g.GenerateExpression()
	return fmt.Sprintf("  t = async(fun() { %s })\n  await(t)", body)
}

func (g *AsyncGenerator) GenerateTaskChain() string {
	// t1 = async(...)
	// t2 = taskFlatMap(t1, fun(x) { async(...) })
	// await(t2)
	depth := g.Src().Intn(5) + 2
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("  t0 = taskResolve(%s)\n", g.GenerateLiteral()))

	for i := 0; i < depth; i++ {
		sb.WriteString(fmt.Sprintf("  t%d = taskFlatMap(t%d, fun(x) { async(fun() { x + 1 }) })\n", i+1, i))
	}
	sb.WriteString(fmt.Sprintf("  await(t%d)", depth))
	return sb.String()
}

func (g *AsyncGenerator) GenerateParallelMap() string {
	// tasks = map(fun(i) { async(...) }, range(0, 100))
	// awaitAll(tasks)
	count := g.Src().Intn(100) + 10 // 10 to 110 tasks
	return fmt.Sprintf("  tasks = map(fun(i) { async(fun() { i * 2 }) }, range(0, %d))\n  awaitAll(tasks)", count)
}

func (g *AsyncGenerator) GenerateRecursiveAsync() string {
	// fun rec(n) { if n == 0 { taskResolve(0) } else { taskFlatMap(async(fun() { rec(n-1) }), fun(t) { await(t) }) } }
	// This is tricky to generate correctly as a one-liner or block.
	// Let's generate a helper function definition first?
	// The Generator structure assumes we are inside main() or top-level.
	// Let's just generate a local recursive function if possible, or use a fixed pattern.

	// Funxy supports local functions.
	depth := g.Src().Intn(20) + 5
	return fmt.Sprintf(`
  fun rec_task(n) {
    if n <= 0 {
      taskResolve(0)
    } else {
      taskFlatMap(async(fun() { n }), fun(x) {
        rec_task(n - 1)
      })
    }
  }
  await(rec_task(%d))`, depth)
}

func (g *AsyncGenerator) GenerateRaceSimulation() string {
	// Simulate a race by spawning two tasks that return different values
	// and awaiting the first one.
	return `
  t1 = async(fun() { 1 })
  t2 = async(fun() { 2 })
  awaitFirst([t1, t2])`
}
