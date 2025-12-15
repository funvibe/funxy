# 04. ADT and Trees

## Task
Model complex data structures using algebraic types.

## What is ADT?

Algebraic Data Types - types composed of:
- Sum types (OR): `A | B | C` - value can be A or B or C
- Product types (AND): `(A, B, C)` or `{ a: A, b: B }` - contains A and B and C

## Simple Sum Types

```rust
// Traffic light: one of three states
type Light = Red | Yellow | Green

fun nextLight(l: Light) -> Light {
    match l {
        Red -> Green
        Green -> Yellow
        Yellow -> Red
    }
}

print(nextLight(Red))  // Green
```

## Sum Types with data

```rust
// Shape can be different form with different parameters
type Shape = Circle(Float) | Rectangle((Float, Float))

fun area(s: Shape) -> Float {
    match s {
        Circle(r) -> 3.14159 * r * r
        Rectangle((w, h)) -> w * h
    }
}

print(area(Circle(5.0)))              // 78.53975
print(area(Rectangle((4.0, 3.0))))    // 12.0
```

## Recursive types: Linked list

```rust
// Own list type
type IntList = ListEnd | ListCons((Int, IntList))

fun listLength(xs: IntList) -> Int {
    match xs {
        ListEnd -> 0
        ListCons((_, tail)) -> 1 + listLength(tail)
    }
}

fun listSum(xs: IntList) -> Int {
    match xs {
        ListEnd -> 0
        ListCons((head, tail)) -> head + listSum(tail)
    }
}

myList = ListCons((1, ListCons((2, ListCons((3, ListEnd))))))
print(listLength(myList))  // 3
print(listSum(myList))     // 6
```

## Binary tree

```rust
type Tree = Leaf(Int) | Node((Tree, Tree))

fun treeSize(t: Tree) -> Int {
    match t {
        Leaf(_) -> 1
        Node((l, r)) -> treeSize(l) + treeSize(r)
    }
}

fun treeDepth(t: Tree) -> Int {
    match t {
        Leaf(_) -> 1
        Node((l, r)) -> {
            dl = treeDepth(l)
            dr = treeDepth(r)
            if dl > dr { 1 + dl } else { 1 + dr }
        }
    }
}

fun treeMap(t: Tree, f) {
    match t {
        Leaf(x) -> Leaf(f(x))
        Node((l, r)) -> Node((treeMap(l, f), treeMap(r, f)))
    }
}

tree = Node((
    Node((Leaf(1), Leaf(2))),
    Leaf(3)
))

print(treeSize(tree))   // 3
print(treeDepth(tree))  // 3
print(treeMap(tree, fun(x) -> x * 10))
```

## BST (Binary Search Tree)

```rust
type BST = BSTEmpty | BSTNode((Int, BST, BST))

fun bstInsert(tree: BST, value: Int) -> BST {
    match tree {
        BSTEmpty -> BSTNode((value, BSTEmpty, BSTEmpty))
        BSTNode((v, left, right)) -> {
            if value < v { BSTNode((v, bstInsert(left, value), right)) }
            else if value > v { BSTNode((v, left, bstInsert(right, value))) }
            else { tree }
        }
    }
}

fun bstContains(tree: BST, value: Int) -> Bool {
    match tree {
        BSTEmpty -> false
        BSTNode((v, left, right)) -> {
            if value == v { true }
            else if value < v { bstContains(left, value) }
            else { bstContains(right, value) }
        }
    }
}

fun bstInOrder(tree: BST) -> List<Int> {
    match tree {
        BSTEmpty -> []
        BSTNode((v, left, right)) -> bstInOrder(left) ++ [v] ++ bstInOrder(right)
    }
}

// Usage
bst = BSTEmpty
    |> fun(t) -> bstInsert(t, 5)
    |> fun(t) -> bstInsert(t, 3)
    |> fun(t) -> bstInsert(t, 7)
    |> fun(t) -> bstInsert(t, 1)
    |> fun(t) -> bstInsert(t, 9)

print(bstInOrder(bst))      // [1, 3, 5, 7, 9]
print(bstContains(bst, 7))  // true
print(bstContains(bst, 4))  // false
```

## Expressions (Expression Tree)

```rust
import "lib/map" (mapGetOr)

type Expr = Num(Int)
          | Add((Expr, Expr))
          | Mul((Expr, Expr))
          | Var(String)

fun eval(expr: Expr, env) -> Int {
    match expr {
        Num(n) -> n
        Add((a, b)) -> eval(a, env) + eval(b, env)
        Mul((a, b)) -> eval(a, env) * eval(b, env)
        Var(name) -> mapGetOr(env, name, 0)
    }
}

fun showExpr(expr: Expr) -> String {
    match expr {
        Num(n) -> show(n)
        Add((a, b)) -> "(" ++ showExpr(a) ++ " + " ++ showExpr(b) ++ ")"
        Mul((a, b)) -> "(" ++ showExpr(a) ++ " * " ++ showExpr(b) ++ ")"
        Var(name) -> name
    }
}

// (2 + x) * 3
expr = Mul((Add((Num(2), Var("x"))), Num(3)))
env = %{ "x" => 5 }

print(showExpr(expr))      // ((2 + x) * 3)
print(eval(expr, env))     // 21
```

## File system

```rust
import "lib/list" (map, foldl, flatten)

type FSEntry = File((String, Int))
             | Directory((String, List<FSEntry>))

fun totalSize(entry: FSEntry) -> Int {
    match entry {
        File((_, size)) -> size
        Directory((_, children)) -> foldl(fun(acc, c) -> acc + totalSize(c), 0, children)
    }
}

fun findFiles(entry: FSEntry, predicate) -> List<String> {
    match entry {
        File((name, size)) -> if predicate(name, size) { [name] } else { [] }
        Directory((_, children)) -> flatten(map(fun(c) -> findFiles(c, predicate), children))
    }
}

fun printTree(entry: FSEntry, indent: String) {
    match entry {
        File((name, size)) -> print(indent ++ name ++ " (" ++ show(size) ++ " bytes)")
        Directory((name, children)) -> {
            print(indent ++ name ++ "/")
            for child in children {
                printTree(child, indent ++ "  ")
            }
        }
    }
}

fs = Directory(("root", [
    File(("readme.txt", 100)),
    Directory(("src", [
        File(("main.lang", 500)),
        File(("utils.lang", 200))
    ])),
    Directory(("docs", [
        File(("api.md", 1500))
    ]))
]))

printTree(fs, "")

print("Total size: " ++ show(totalSize(fs)))  // 2300

largeFiles = findFiles(fs, fun(name, size) -> size > 300)
print(largeFiles)  // ["main.lang", "api.md"]
