package modules

import (
	"github.com/funvibe/funxy/internal/typesystem"
)

// initRpcPackage registers the lib/rpc virtual package
func initRpcPackage() {
	stringType := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{typesystem.Char},
	}

	intType := typesystem.Int

	// Result can return any type from RPC, but we type it generally as Result<String, b>
	resultAny := typesystem.TApp{
		Constructor: ResultCon,
		Args:        []typesystem.Type{stringType, typesystem.TVar{Name: "b"}},
	}

	pkg := &VirtualPackage{
		Name: "rpc",
		Symbols: map[string]typesystem.Type{
			"callWait": typesystem.TFunc{
				Params:       []typesystem.Type{stringType, stringType, typesystem.TVar{Name: "a"}, intType},
				ReturnType:   resultAny,
				DefaultCount: 1, // timeoutMs is optional
			},
			"callWaitGroup": typesystem.TFunc{
				Params:       []typesystem.Type{stringType, stringType, typesystem.TVar{Name: "a"}, intType},
				ReturnType:   resultAny,
				DefaultCount: 1, // timeoutMs is optional
			},
		},
	}
	RegisterVirtualPackage("lib/rpc", pkg)
}
