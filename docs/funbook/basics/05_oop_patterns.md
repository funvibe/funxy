# 05. OOP Patterns in Funxy

## For those accustomed to classes and objects

Funxy doesn't have classes in the traditional sense, but all familiar patterns are easily implemented - often simpler and safer.

---

## Records = Data Classes

```rust
// Java/Kotlin: data class User(val name: String, val age: Int)
// Python: @dataclass class User

// Funxy:
type User = { name: String, age: Int, email: String }

// Creating an "object"
user = { name: "Alice", age: 30, email: "alice@example.com" }

// Accessing fields
print(user.name)  // Alice

// Immutable update (like copy() in Kotlin)
older = { ...user, age: user.age + 1 }
print(older.age)  // 31
```

---

## Traits = Interfaces

```rust
// Java: interface Formatter { String format(); }
// Funxy:
trait Formatter<T> {
    fun format(self: T) -> String
}

// "Class" User
type User = { name: String, age: Int }

// Interface implementation
instance Formatter User {
    fun format(self: User) -> String {
        "User(" ++ self.name ++ ", " ++ show(self.age) ++ ")"
    }
}

// "Class" Product
type Product = { name: String, price: Float }

instance Formatter Product {
    fun format(self: Product) -> String {
        self.name ++ ": $" ++ show(self.price)
    }
}

// Polymorphism!
fun printItem<T: Formatter>(item: T) -> Nil { print(format(item)) }

u: User = { name: "Alice", age: 30 }
p: Product = { name: "Book", price: 19.99 }
printItem(u)  // User(Alice, 30)
printItem(p)  // Book: $19.99
```

---

## Methods on types

```rust
// Java: class User { String greet() { return "Hi, " + name; } }

// Funxy - functions that take the type as the first argument
type User = { name: String, age: Int }

fun greet(user: User) -> String { "Hi, I'm " ++ user.name }

fun isAdult(user: User) -> Bool { user.age >= 18 }

fun haveBirthday(user: User) -> User { { ...user, age: user.age + 1 } }

// Usage (like methods through pipe)
alice = { name: "Alice", age: 30 }

print(alice |> greet)       // Hi, I'm Alice
print(alice |> isAdult)     // true

older = alice |> haveBirthday
print(older.age)            // 31
```

---

## Encapsulation through modules

Funxy uses modules for encapsulation. Only needed symbols are exported.

**counter.lang:**
```rust
// Module counter
module counter

// Private type (not exported)
type CounterState = { value: Int }

// Public "constructor"
export fun newCounter(initial: Int) -> CounterState { { value: initial } }

// Public "methods"  
export fun increment(c: CounterState) -> CounterState { { value: c.value + 1 } }
export fun decrement(c: CounterState) -> CounterState { { value: c.value - 1 } }
export fun getValue(c: CounterState) -> Int { c.value }
// ...
```

**main.lang:**
```rust
import "counter" (newCounter, increment, getValue)

c = newCounter(0)
c2 = c |> increment |> increment |> increment
print(getValue(c2))  // 3
// ...
```

---

## ADT = Sealed Classes / Enums with data

```rust
// Kotlin: sealed class Shape
// Java 17+: sealed interface Shape permits Circle, Rectangle

// Funxy:
type Shape = Circle(Float) | Rectangle((Float, Float))

// The "visitor" pattern is built into the language!
fun area(shape: Shape) -> Float {
    match shape {
        Circle(r) -> 3.14159 * r * r
        Rectangle((w, h)) -> w * h
    }
}

fun describe(shape: Shape) -> String {
    match shape {
        Circle(r) -> "Circle with radius " ++ show(r)
        Rectangle((w, h)) -> "Rectangle " ++ show(w) ++ "x" ++ show(h)
    }
}

// Usage
shapes = [Circle(5.0), Rectangle((4.0, 3.0))]

for s in shapes {
    print(describe(s) ++ " has area " ++ show(area(s)))
}
// Circle with radius 5 has area 78.53975
// Rectangle 4x3 has area 12
```

---

## Builder pattern

```rust
// Java: new UserBuilder().name("Alice").age(30).build()

// Funxy - just record updates
type User = { name: String, age: Int, email: String, role: String }

// "Builder" - just defaults + update
defaultUser :- { name: "", age: 0, email: "", role: "user" }

fun withName(u: User, name: String) -> User { { ...u, name: name } }
fun withAge(u: User, age: Int) -> User { { ...u, age: age } }
fun withEmail(u: User, email: String) -> User { { ...u, email: email } }
fun withRole(u: User, role: String) -> User { { ...u, role: role } }

// Fluent API through pipe
admin = defaultUser
    |> fun(u) -> withName(u, "Alice")
    |> fun(u) -> withAge(u, 30)
    |> fun(u) -> withEmail(u, "alice@example.com")
    |> fun(u) -> withRole(u, "admin")

print(admin.name)   // Alice
print(admin.role)   // admin
```

---

## Factory pattern

```rust
type Shape = Circle(Float) | Rectangle((Float, Float))

// Factory functions
fun createCircle(radius: Float) -> Shape { Circle(radius) }
fun createSquare(side: Float) -> Shape { Rectangle((side, side)) }
fun createRectangle(w: Float, h: Float) -> Shape { Rectangle((w, h)) }

// Factory with validation
fun createValidCircle(radius: Float) -> Result<String, Shape> {
    if radius <= 0.0 { Fail("Radius must be positive") }
    else { Ok(Circle(radius)) }
}

print(createValidCircle(5.0))   // Ok(Circle(5))
print(createValidCircle(-1.0))  // Fail("Radius must be positive")
```

---

## Strategy pattern

```rust
// Java: interface Strategy { int execute(int a, int b); }

// Funxy - just functions!
type Strategy = (Int, Int) -> Int

fun add(a: Int, b: Int) -> Int { a + b }
fun multiply(a: Int, b: Int) -> Int { a * b }
fun power(a: Int, b: Int) -> Int { a ** b }

// Context
fun executeStrategy(strategy: Strategy, a: Int, b: Int) -> Int {
    strategy(a, b)
}

// Usage
print(executeStrategy(add, 5, 3))       // 8
print(executeStrategy(multiply, 5, 3))  // 15
print(executeStrategy(power, 5, 3))     // 125

// Or even simpler - pass a lambda
print(executeStrategy(fun(a, b) -> a - b, 5, 3))  // 2
```

---

## Observer pattern

```rust
import "lib/list" (forEach)

// Type alias for function type
type Observer = (Int) -> Nil
type Subject = { observers: List<Observer>, value: Int }

fun createSubject(initial: Int) -> Subject { 
    { observers: [], value: initial }
}

fun subscribe(subject: Subject, observer: Observer) -> Subject {
    { ...subject, observers: subject.observers ++ [observer] }
}

fun notify(subject: Subject) -> Nil {
    forEach(fun(obs) -> obs(subject.value), subject.observers)
}

fun setValue(subject: Subject, value: Int) -> Subject {
    updated = { ...subject, value: value }
    notify(updated)
    updated
}

// Usage
subject = createSubject(0)
s1 = subscribe(subject, fun(v) -> print("Observer 1: " ++ show(v)))
s2 = subscribe(s1, fun(v) -> print("Observer 2: " ++ show(v)))

_ = setValue(s2, 42)
// Observer 1: 42
// Observer 2: 42
```

---

## State through closures

```rust
// Encapsulated mutable state
fun createCounter() {
    count = 0
    {
        get: fun() -> count,
        inc: fun() { count += 1 },
        dec: fun() { count -= 1 }
    }
}

counter = createCounter()
counter.inc()
counter.inc()
counter.inc()
print(counter.get())  // 3
counter.dec()
print(counter.get())  // 2
```

---

## Inheritance? Composition!

```rust
import "lib/list" (contains)

// OOP: class Admin extends User
// Funxy: composition instead of inheritance

type User = { name: String, email: String }
type Admin = { user: User, permissions: List<String> }

// Functions for User
fun greetUser(u: User) -> String { "Hello, " ++ u.name }

// Admin can use User functions
fun greetAdmin(a: Admin) -> String { greetUser(a.user) ++ " (Admin)" }

fun hasPermission(a: Admin, perm: String) -> Bool {
    contains(a.permissions, perm)
}

// Creation
admin = {
    user: { name: "Alice", email: "alice@example.com" },
    permissions: ["read", "write", "delete"]
}

print(greetAdmin(admin))                    // Hello, Alice (Admin)
print(hasPermission(admin, "delete"))       // true
```

---

## Paradigm comparison

| OOP concept | Funxy equivalent |
|-------------|-----------------|
| Class | `type` (record) |
| Interface | `trait` |
| Method | Function with type as first argument |
| Inheritance | Record composition |
| Private fields | Modules (export only needed) |
| Sealed class | ADT (sum types) |
| Factory | Constructor function |
| Builder | Pipe + update functions |
| Strategy | First-class functions |
| Singleton | Constant in module |

---

## Advantages of the Funxy approach

1. Immutability by default - no "accidental" mutations
2. Exhaustive matching - compiler checks all ADT cases
3. No null pointer exceptions - Option/Result types
4. Simplicity - less boilerplate, more essence
5. Composition > inheritance - more flexible and clearer
