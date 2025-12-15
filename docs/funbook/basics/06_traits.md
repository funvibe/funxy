# Traits and Operators

## Basic trait

```rust
// Trait declaration
trait Formatter<T> {
    fun format(val: T) -> String
}

// ADT type
type UserId = MkUserId(Int)

instance Formatter UserId {
    fun format(val: UserId) -> String {
        match val { MkUserId(n) -> "User#" ++ show(n) }
    }
}

uid = MkUserId(42)
print(format(uid))  // User#42
```

---

## Super traits (inheritance)

A trait can inherit from other traits:

```rust
type Ordering = Lt | Eq | Gt

// Base trait
trait MyEq<T> {
    fun myEq(a: T, b: T) -> Bool
}

// MyOrd inherits MyEq
trait MyOrd<T> : MyEq<T> {
    fun myCmp(a: T, b: T) -> Ordering
}

// ADT for money
type Money = MkMoney(Int)

fun getAmount(m: Money) -> Int {
    match m { MkMoney(n) -> n }
}

// First MyEq
instance MyEq Money {
    fun myEq(a: Money, b: Money) -> Bool {
        getAmount(a) == getAmount(b)
    }
}

// Then MyOrd (requires MyEq)
instance MyOrd Money {
    fun myCmp(a: Money, b: Money) -> Ordering {
        if getAmount(a) < getAmount(b) { Lt }
        else if getAmount(a) > getAmount(b) { Gt }
        else { Eq }
    }
}

m1 = MkMoney(100)
m2 = MkMoney(200)
print(myEq(m1, m1))   // true
print(myCmp(m1, m2))  // Lt
```

---

## Operator overloading

To use standard operators, you need to implement the corresponding built-in traits (e.g., `Equal`, `Numeric`, `Semigroup`).

```rust
// ADT for 2D vector
type Vec2 = MkVec2((Float, Float))

fun getX(v: Vec2) -> Float { match v { MkVec2((x, _)) -> x } }
fun getY(v: Vec2) -> Float { match v { MkVec2((_, y)) -> y } }

// Implement Equal for == and != operators
instance Equal Vec2 {
    operator (==)(a: Vec2, b: Vec2) -> Bool {
        getX(a) == getX(b) && getY(a) == getY(b)
    }
    // != is implemented by default through !(a == b)
}

// Implement Semigroup for <> operator (combination/addition)
// We use <> instead of +, as Numeric requires implementation of all arithmetic operations
instance Semigroup Vec2 {
    operator (<>)(a: Vec2, b: Vec2) -> Vec2 {
        MkVec2((getX(a) + getX(b), getY(a) + getY(b)))
    }
}

v1 = MkVec2((1.0, 2.0))
v2 = MkVec2((3.0, 4.0))

// Using operators
v3 = v1 <> v2       // Vector addition through Semigroup
print(v3)           // MkVec2((4.0, 6.0))

print(v1 == v1)     // true
print(v1 != v2)     // true
```

---

## Operator $ (application)

Low-priority function application:

```rust
fun double(x: Int) -> Int { x * 2 }
fun inc(x: Int) -> Int { x + 1 }

// f $ x = f(x)
print(double $ 21)  // 42

// Right-associative: f $ g $ x = f(g(x))
print(inc $ double $ 5)  // 11 = inc(double(5))

// Convenient for avoiding parentheses
print $ double $ inc $ 10  // 22
```

---

## Operators as functions

Any operator can be used as a function:

```rust
import "lib/list" (foldl)

// Operator in a variable
add = (+)
print(add(1, 2))  // 3

// In higher-order functions
sum = foldl((+), 0, [1, 2, 3, 4, 5])
print(sum)  // 15

product = foldl((*), 1, [1, 2, 3, 4])
print(product)  // 24
```

---

## Constraints

```rust
trait Showable<T> {
    fun render(val: T) -> String
}

instance Showable Int {
    fun render(val: Int) -> String { "Int:" ++ show(val) }
}

// Function requires Showable
fun displayValue<T: Showable>(x: T) -> String {
    render(x)
}

print(displayValue(42))  // Int:42
```

---

## Default implementations

```rust
trait MathOps<T> {
    fun mathAdd(a: T, b: T) -> T
    fun mathMul(a: T, b: T) -> T

    // Default implementation
    fun mathSquare(x: T) -> T {
        mathMul(x, x)
    }

    fun mathDouble(x: T) -> T {
        mathAdd(x, x)
    }
}

type MyNum = MkMyNum(Int)

fun getVal(n: MyNum) -> Int { match n { MkMyNum(v) -> v } }

instance MathOps MyNum {
    fun mathAdd(a: MyNum, b: MyNum) -> MyNum {
        MkMyNum(getVal(a) + getVal(b))
    }
    fun mathMul(a: MyNum, b: MyNum) -> MyNum {
        MkMyNum(getVal(a) * getVal(b))
    }
    // mathSquare and mathDouble work automatically!
}

n = MkMyNum(5)
print(mathSquare(n))  // MkMyNum(25)
print(mathDouble(n))  // MkMyNum(10)
```

---

## Custom operators (UserOp)

Available slots for custom operators:

| Operator | Trait | Associativity | Typical usage |
|----------|-------|-----------------|------------------------|
| `<>` | `Semigroup` | Right | Combination (Semigroup) |
| `<|>` | `UserOpChoose` | Left | Alternative |
| `<:>` | `UserOpCons` | Right | Cons-like prepend |
| `<~>` | `UserOpSwap` | Left | Exchange/Swap |
| `<$>` | `UserOpMap` | Left | Functor map |
| `=>` | `UserOpImply` | Right | Implication |
| `<|` | `UserOpPipeLeft` | Right | Right pipe |
| `$` | (built-in) | Right | Function application |

### Semigroup (<>)

```rust
type Text = MkText(String)

fun getText(t: Text) -> String { match t { MkText(s) -> s } }

instance Semigroup Text {
    operator (<>)(a: Text, b: Text) -> Text {
        match (a, b) { (MkText(x), MkText(y)) -> MkText(x ++ y) }
    }
}

t1 = MkText("Hello")
t2 = MkText(" ")
t3 = MkText("World")
result = t1 <> t2 <> t3  // right-associative
print(getText(result))   // Hello World
```

### UserOpChoose (<|>) — Alternative

```rust
type Maybe = MkJust(Int) | MkNothing

fun getMaybe(m: Maybe) -> Int {
    match m {
        MkJust(x) -> x
        MkNothing -> -1
    }
}

instance UserOpChoose Maybe {
    operator (<|>)(a: Maybe, b: Maybe) -> Maybe {
        match a {
            MkJust(_) -> a
            MkNothing -> b
        }
    }
}

m1 = MkNothing
m2 = MkJust(42)
print(getMaybe(m1 <|> m2))  // 42 (first non-Nothing)
```

### UserOpImply (=>) — Implication

```rust
type Logic = MkLogic(Bool)

fun getLogic(l: Logic) -> Bool { match l { MkLogic(b) -> b } }

instance UserOpImply Logic {
    operator (=>)(a: Logic, b: Logic) -> Logic {
        // a => b is equivalent to !a || b
        match (a, b) {
            (MkLogic(x), MkLogic(y)) -> MkLogic(!x || y)
        }
    }
}

lt = MkLogic(true)
lf = MkLogic(false)
print(getLogic(lt => lt))  // true  (true => true)
print(getLogic(lt => lf))  // false (true => false)
print(getLogic(lf => lt))  // true  (false => anything)
```

### Operators as functions

```rust
type Text = MkText(String)
fun getText(t: Text) -> String { match t { MkText(s) -> s } }

instance Semigroup Text {
    operator (<>)(a: Text, b: Text) -> Text {
        match (a, b) { (MkText(x), MkText(y)) -> MkText(x ++ y) }
    }
}

// Operator in a variable
combine = (<>)
t4 = combine(MkText("A"), MkText("B"))
print(getText(t4))  // AB
```

---

## Supported operators

| Operator | Description |
|----------|----------|
| `+`, `-`, `*`, `/`, `%`, `**` | Arithmetic |
| `==`, `!=` | Equality |
| `<`, `>`, `<=`, `>=` | Comparison |
| `&`, `|`, `^`, `<<`, `>>` | Bitwise |
| `++` | Concatenation |
| `::` | Cons (prepend) |
| `|>` | Pipe |
| `$` | Function application |
| `,,` | Composition |
| `<>` | Semigroup combine |
| `<|>` | Alternative choice |
| `<:>` | Custom cons |
| `<~>` | Swap |
| `<$>` | Functor map |
| `=>` | Implication |
