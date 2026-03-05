package modules

import (
	"github.com/funvibe/funxy/internal/typesystem"
)

// initSupervisorPackage registers the lib/vmm virtual package
func initSupervisorPackage() {
	stringType := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{typesystem.Char},
	}
	listString := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{stringType},
	}
	// Config is a generic record
	configType := typesystem.TVar{Name: "config"}

	resultString := typesystem.TApp{
		Constructor: ResultCon,
		Args:        []typesystem.Type{stringType, stringType},
	}

	resultListString := typesystem.TApp{
		Constructor: ResultCon,
		Args:        []typesystem.Type{stringType, listString},
	}

	mapStringInt := typesystem.TApp{
		Constructor: MapCon,
		Args:        []typesystem.Type{stringType, typesystem.Int},
	}
	circuitRecord := typesystem.TRecord{
		Fields: map[string]typesystem.Type{
			"vmId":                     stringType,
			"state":                    stringType,
			"stateCode":                typesystem.Int,
			"failureCount":             typesystem.Int,
			"openSinceMs":              typesystem.Int,
			"halfOpenInFlight":         typesystem.Bool,
			"fastFailTotal":            typesystem.Int,
			"transitionsOpenTotal":     typesystem.Int,
			"transitionsHalfOpenTotal": typesystem.Int,
			"transitionsClosedTotal":   typesystem.Int,
			"failureThreshold":         typesystem.Int,
			"failureWindowMs":          typesystem.Int,
			"openTimeoutMs":            typesystem.Int,
		},
		IsOpen: true,
	}

	eventRecord := typesystem.TRecord{
		Fields: map[string]typesystem.Type{
			"type": stringType,
			"vmId": stringType,
			"seq":  typesystem.Int,
		},
		IsOpen: true,
	}

	pkg := &VirtualPackage{
		Name: "vmm",
		Symbols: map[string]typesystem.Type{
			"spawnVM": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, configType, typesystem.TApp{Constructor: typesystem.TCon{Name: "Option"}, Args: []typesystem.Type{typesystem.TRecord{Fields: map[string]typesystem.Type{}, IsOpen: true}}}},
				ReturnType: resultString,
				IsVariadic: true,
			},
			"spawnVMGroup": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, configType, typesystem.Int, typesystem.TApp{Constructor: typesystem.TCon{Name: "Option"}, Args: []typesystem.Type{typesystem.TRecord{Fields: map[string]typesystem.Type{}, IsOpen: true}}}},
				ReturnType: resultListString,
				IsVariadic: true,
			},
			"killVM":   typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: typesystem.Nil},
			"traceOn":  typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: typesystem.Nil, DefaultCount: 1},
			"traceOff": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: typesystem.Nil, DefaultCount: 1},
			"stopVM": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, typesystem.TApp{Constructor: typesystem.TCon{Name: "Option"}, Args: []typesystem.Type{typesystem.TVar{Name: "record"}}}},
				ReturnType: typesystem.TApp{Constructor: typesystem.TCon{Name: "Result"}, Args: []typesystem.Type{stringType, typesystem.TVar{Name: "a"}}},
			},
			"listVMs": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: listString},
			"vmStats": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: mapStringInt},
			"rpcCircuitStats": typesystem.TFunc{
				Params:     []typesystem.Type{stringType},
				ReturnType: circuitRecord,
			},
			"receiveEventWait": typesystem.TFunc{
				Params:       []typesystem.Type{typesystem.Int},
				ReturnType:   eventRecord,
				DefaultCount: 1,
			},
			"serialize": typesystem.TFunc{
				Params:       []typesystem.Type{typesystem.TVar{Name: "a"}, stringType},
				ReturnType:   typesystem.TCon{Name: "Bytes"},
				DefaultCount: 1,
			},
			"deserialize": typesystem.TFunc{
				Params:     []typesystem.Type{typesystem.TCon{Name: "Bytes"}},
				ReturnType: typesystem.TApp{Constructor: typesystem.TCon{Name: "Result"}, Args: []typesystem.Type{stringType, typesystem.TVar{Name: "a"}}},
			},
			"getState": typesystem.TFunc{
				Params:     []typesystem.Type{},
				ReturnType: typesystem.TApp{Constructor: typesystem.TCon{Name: "Option"}, Args: []typesystem.Type{typesystem.TVar{Name: "a"}}},
			},
			"setState": typesystem.TFunc{Params: []typesystem.Type{typesystem.TVar{Name: "a"}}, ReturnType: typesystem.Nil},
		},
	}
	RegisterVirtualPackage("lib/vmm", pkg)
}
