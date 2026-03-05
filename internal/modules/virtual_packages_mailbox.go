package modules

import (
	"github.com/funvibe/funxy/internal/typesystem"
)

// initMailboxPackage registers the lib/mailbox virtual package
func initMailboxPackage() {
	stringType := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{typesystem.Char},
	}

	resultStringNil := typesystem.TApp{
		Constructor: ResultCon,
		Args:        []typesystem.Type{stringType, typesystem.Nil},
	}

	resultStringMsg := typesystem.TApp{
		Constructor: ResultCon,
		Args:        []typesystem.Type{stringType, typesystem.TVar{Name: "msg"}},
	}

	msgRecord := typesystem.TVar{Name: "msg"}

	predicateType := typesystem.TFunc{
		Params:     []typesystem.Type{typesystem.TVar{Name: "msg"}},
		ReturnType: typesystem.Bool,
	}

	importanceType := typesystem.TCon{Name: "Importance"}

	pkg := &VirtualPackage{
		Name: "mailbox",
		Types: map[string]typesystem.Type{
			"Importance": importanceType,
		},
		Constructors: map[string]typesystem.Type{
			"Low":    importanceType,
			"Info":   importanceType,
			"Warn":   importanceType,
			"Crit":   importanceType,
			"System": importanceType,
		},
		Variants: map[string][]string{
			"Importance": {"Low", "Info", "Warn", "Crit", "System"},
		},
		Symbols: map[string]typesystem.Type{
			"send": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, msgRecord},
				ReturnType: resultStringNil,
			},
			"sendWait": typesystem.TFunc{
				Params:       []typesystem.Type{stringType, msgRecord, typesystem.Int},
				DefaultCount: 1,
				ReturnType:   resultStringNil,
			},
			"reply": typesystem.TFunc{
				Params: []typesystem.Type{
					typesystem.TVar{Name: "r"}, // original message record
					typesystem.TVar{Name: "a"}, // payload
				},
				ReturnType: resultStringNil,
			},
			"replyWait": typesystem.TFunc{
				Params: []typesystem.Type{
					typesystem.TVar{Name: "r"}, // original message record
					typesystem.TVar{Name: "a"}, // payload
					typesystem.Int,
				},
				DefaultCount: 1,
				ReturnType:   resultStringNil,
			},
			"requestWait": typesystem.TFunc{
				Params: []typesystem.Type{
					stringType,
					typesystem.TVar{Name: "a"}, // payload can be anything
					typesystem.Int,
				},
				DefaultCount: 1,
				ReturnType:   resultStringMsg, // returns a message record
			},
			"receive": typesystem.TFunc{
				Params:     []typesystem.Type{},
				ReturnType: resultStringMsg,
			},
			"receiveWait": typesystem.TFunc{
				Params:       []typesystem.Type{typesystem.Int},
				DefaultCount: 1,
				ReturnType:   resultStringMsg,
			},
			"receiveBy": typesystem.TFunc{
				Params:     []typesystem.Type{predicateType},
				ReturnType: resultStringMsg,
			},
			"receiveByWait": typesystem.TFunc{
				Params:       []typesystem.Type{predicateType, typesystem.Int},
				DefaultCount: 1,
				ReturnType:   resultStringMsg,
			},
			"peek": typesystem.TFunc{
				Params:     []typesystem.Type{},
				ReturnType: resultStringMsg,
			},
			"peekBy": typesystem.TFunc{
				Params:     []typesystem.Type{predicateType},
				ReturnType: resultStringMsg,
			},
		},
	}
	RegisterVirtualPackage("lib/mailbox", pkg)
}
