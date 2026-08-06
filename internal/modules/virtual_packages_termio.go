package modules

import "github.com/funvibe/funxy/internal/typesystem"

// initTermIOPackage registers the lib/termio virtual package.
func initTermIOPackage() {
	stringType := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{typesystem.Char},
	}
	inputEventType := typesystem.TCon{Name: "InputEvent"}
	optionInputEvent := typesystem.TApp{
		Constructor: OptionCon,
		Args:        []typesystem.Type{inputEventType},
	}
	a := typesystem.TVar{Name: "A"}

	pkg := &VirtualPackage{
		Name: "termio",
		Types: map[string]typesystem.Type{
			"InputEvent": inputEventType,
		},
		Constructors: map[string]typesystem.Type{
			"KeyEvent": typesystem.TFunc{
				Params:     []typesystem.Type{stringType},
				ReturnType: inputEventType,
			},
			"TextEvent": typesystem.TFunc{
				Params:     []typesystem.Type{stringType},
				ReturnType: inputEventType,
			},
		},
		Variants: map[string][]string{
			"InputEvent": {"KeyEvent", "TextEvent"},
		},
		Symbols: map[string]typesystem.Type{
			"withTerminalInput": typesystem.TFunc{
				Params: []typesystem.Type{
					typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: a},
				},
				ReturnType: a,
			},
			"readInputEvent": typesystem.TFunc{
				Params:       []typesystem.Type{typesystem.Int},
				ReturnType:   optionInputEvent,
				DefaultCount: 1,
			},
		},
	}

	RegisterVirtualPackage("lib/termio", pkg)
}
