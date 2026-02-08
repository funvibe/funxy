package evaluator

import (
	"fmt"
	"github.com/funvibe/funxy/internal/typesystem"
	"os"
	"strconv"
	"strings"
)

// Flag definitions
type flagDef struct {
	name         string
	defaultValue Object
	description  string
	kind         string // "bool", "int", "float", "string"
}

// Global state for flags
var (
	flagRegistry  = make(map[string]*flagDef)
	parsedFlags   = make(map[string]Object)
	remainingArgs []string
)

// FlagBuiltins returns built-in functions for lib/flag virtual package
func FlagBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		"flagSet":   {Fn: builtinFlagSet, Name: "flagSet"},
		"flagParse": {Fn: builtinFlagParse, Name: "flagParse"},
		"flagGet":   {Fn: builtinFlagGet, Name: "flagGet"},
		"flagArgs":  {Fn: builtinFlagArgs, Name: "flagArgs"},
		"flagUsage": {Fn: builtinFlagUsage, Name: "flagUsage"},
	}
}

// SetFlagBuiltinTypes sets type info for flag builtins
func SetFlagBuiltinTypes(builtins map[string]*Builtin) {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}
	// List<String>
	listString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{stringType},
	}

	// Generic T
	T := typesystem.TVar{Name: "T"}

	types := map[string]typesystem.Type{
		"flagSet": typesystem.TFunc{
			Params:     []typesystem.Type{stringType, T, stringType},
			ReturnType: typesystem.Nil,
		},
		"flagParse": typesystem.TFunc{
			Params:       []typesystem.Type{listString},
			ReturnType:   typesystem.Nil,
			DefaultCount: 1, // args is optional
		},
		"flagGet": typesystem.TFunc{
			Params:     []typesystem.Type{stringType},
			ReturnType: T,
		},
		"flagArgs": typesystem.TFunc{
			Params:     []typesystem.Type{},
			ReturnType: listString,
		},
		"flagUsage": typesystem.TFunc{
			Params:     []typesystem.Type{},
			ReturnType: typesystem.Nil,
		},
	}

	for name, typ := range types {
		if b, ok := builtins[name]; ok {
			b.TypeInfo = typ
		}
	}
}

// flagSet(name: String, default: T, desc: String) -> Nil
func builtinFlagSet(e *Evaluator, args ...Object) Object {
	if len(args) != 3 {
		return newError("flagSet expects 3 arguments, got %d", len(args))
	}

	name := listToString(args[0])
	defaultValue := args[1]
	desc := listToString(args[2])

	kind := ""
	switch defaultValue.(type) {
	case *Boolean:
		kind = "bool"
	case *Integer:
		kind = "int"
	case *Float:
		kind = "float"
	case *List: // String
		kind = "string"
	default:
		return newError("unsupported flag type: %s", defaultValue.Type())
	}

	flagRegistry[name] = &flagDef{
		name:         name,
		defaultValue: defaultValue,
		description:  desc,
		kind:         kind,
	}

	return &Nil{}
}

// flagParse(args: List<String>?) -> Nil
func builtinFlagParse(e *Evaluator, args ...Object) Object {
	var inputArgs []string

	if len(args) > 0 {
		if list, ok := args[0].(*List); ok {
			for _, elem := range list.ToSlice() {
				inputArgs = append(inputArgs, listToString(elem))
			}
		} else {
			return newError("flagParse expects List<String> as argument")
		}
	} else {
		// Use os.Args, skipping program name
		if len(os.Args) > 1 {
			inputArgs = os.Args[1:]
		}
	}

	// Reset state
	parsedFlags = make(map[string]Object)
	remainingArgs = []string{}

	// Simple parser
	for i := 0; i < len(inputArgs); i++ {
		arg := inputArgs[i]

		if !strings.HasPrefix(arg, "-") {
			// Positional argument
			remainingArgs = append(remainingArgs, arg)
			continue
		}

		// Handle --name=value or -name=value
		name := strings.TrimLeft(arg, "-")
		value := ""
		hasValue := false

		if strings.Contains(name, "=") {
			parts := strings.SplitN(name, "=", 2)
			name = parts[0]
			value = parts[1]
			hasValue = true
		}

		def, ok := flagRegistry[name]
		if !ok {
			// Unknown flag
			// For now, treat as remaining arg or ignore?
			// Go flag package prints error and exits.
			// Let's print error and return it? Or just print to stderr
			fmt.Fprintf(os.Stderr, "flag provided but not defined: -%s\n", name)
			return builtinFlagUsage(e)
		}

		if def.kind == "bool" {
			if hasValue {
				// --bool=true
				b, err := strconv.ParseBool(value)
				if err == nil {
					parsedFlags[name] = &Boolean{Value: b}
				} else {
					return newError("invalid boolean value %q for -%s", value, name)
				}
			} else {
				// --bool (implies true)
				parsedFlags[name] = &Boolean{Value: true}
			}
		} else {
			// Need a value
			if !hasValue {
				if i+1 < len(inputArgs) {
					value = inputArgs[i+1]
					i++ // Consume next arg
				} else {
					return newError("flag needs an argument: -%s", name)
				}
			}

			switch def.kind {
			case "string":
				parsedFlags[name] = stringToList(value)
			case "int":
				v, err := strconv.ParseInt(value, 10, 64)
				if err != nil {
					return newError("invalid value %q for flag -%s: %v", value, name, err)
				}
				parsedFlags[name] = &Integer{Value: v}
			case "float":
				v, err := strconv.ParseFloat(value, 64)
				if err != nil {
					return newError("invalid value %q for flag -%s: %v", value, name, err)
				}
				parsedFlags[name] = &Float{Value: v}
			}
		}
	}

	return &Nil{}
}

// flagGet(name: String) -> T
func builtinFlagGet(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("flagGet expects 1 argument")
	}
	name := listToString(args[0])

	// Check parsed values first
	if val, ok := parsedFlags[name]; ok {
		return val
	}

	// Fallback to default
	if def, ok := flagRegistry[name]; ok {
		return def.defaultValue
	}

	return newError("flag not defined: %s", name)
}

// flagArgs() -> List<String>
func builtinFlagArgs(e *Evaluator, args ...Object) Object {
	elements := make([]Object, len(remainingArgs))
	for i, s := range remainingArgs {
		elements[i] = stringToList(s)
	}
	return newList(elements)
}

// flagUsage() -> Nil
func builtinFlagUsage(e *Evaluator, args ...Object) Object {
	fmt.Fprintf(os.Stderr, "Usage:\n")
	for name, def := range flagRegistry {
		typeStr := def.kind
		if def.kind == "string" {
			typeStr = "string"
		}

		defStr := def.defaultValue.Inspect()
		// Clean up string default display
		if def.kind == "string" {
			// Remove quotes for display if it's cleaner? No, keep them to show it's a string
		}

		fmt.Fprintf(os.Stderr, "  -%s %s\n    \t%s (default %s)\n", name, typeStr, def.description, defStr)
	}
	return &Nil{}
}
