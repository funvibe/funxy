package modules

import (
	"github.com/funvibe/funxy/internal/typesystem"
)

func initTestPackage() {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{typesystem.Char},
	}

	// Generic type variable
	T := typesystem.TVar{Name: "T"}
	A := typesystem.TVar{Name: "A"}
	E := typesystem.TVar{Name: "E"}

	// Option<T>
	optionT := typesystem.TApp{
		Constructor: OptionCon,
		Args:        []typesystem.Type{T},
	}

	// Result<E, T> - E is error, T is success
	resultET := typesystem.TApp{
		Constructor: ResultCon,
		Args:        []typesystem.Type{E, T},
	}

	// Result<String, A> for mock returns - error is String, success is A
	resultStringA := typesystem.TApp{
		Constructor: ResultCon,
		Args:        []typesystem.Type{stringType, A},
	}

	// HttpResponse type (same as in lib/http)
	headerTuple := typesystem.TTuple{
		Elements: []typesystem.Type{stringType, stringType},
	}
	headersType := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{headerTuple},
	}
	responseType := typesystem.TRecord{
		Fields: map[string]typesystem.Type{
			"status":  typesystem.Int,
			"body":    stringType,
			"headers": headersType,
		},
	}

	// Test body function type: () -> Nil
	testBodyType := typesystem.TFunc{
		Params:     []typesystem.Type{},
		ReturnType: typesystem.Nil,
	}

	pkg := &VirtualPackage{
		Name: "test",
		Symbols: map[string]typesystem.Type{
			// Test definition
			"testRun": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, testBodyType},
				ReturnType: typesystem.Nil,
			},
			"testSkip": typesystem.TFunc{
				Params:     []typesystem.Type{stringType},
				ReturnType: typesystem.Nil,
			},
			"testExpectFail": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, testBodyType},
				ReturnType: typesystem.Nil,
			},

			// Assertions (all accept optional message as last argument)
			"assert": typesystem.TFunc{
				Params:     []typesystem.Type{typesystem.Bool, stringType},
				ReturnType: typesystem.Nil,
				IsVariadic: true,
			},
			"assertTrue": typesystem.TFunc{
				Params:     []typesystem.Type{typesystem.Bool, stringType},
				ReturnType: typesystem.Nil,
				IsVariadic: true,
			},
			"assertFalse": typesystem.TFunc{
				Params:     []typesystem.Type{typesystem.Bool, stringType},
				ReturnType: typesystem.Nil,
				IsVariadic: true,
			},
			"assertEquals": typesystem.TFunc{
				Params:     []typesystem.Type{T, T, stringType},
				ReturnType: typesystem.Nil,
				IsVariadic: true,
			},
			"assertOk": typesystem.TFunc{
				Params:     []typesystem.Type{resultET, stringType},
				ReturnType: typesystem.Nil,
				IsVariadic: true,
			},
			"assertFail": typesystem.TFunc{
				Params:     []typesystem.Type{resultET, stringType},
				ReturnType: typesystem.Nil,
				IsVariadic: true,
			},
			"assertSome": typesystem.TFunc{
				Params:     []typesystem.Type{optionT, stringType},
				ReturnType: typesystem.Nil,
				IsVariadic: true,
			},
			"assertZero": typesystem.TFunc{
				Params:     []typesystem.Type{optionT, stringType},
				ReturnType: typesystem.Nil,
				IsVariadic: true,
			},

			// HTTP mocks
			"mockHttp": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, responseType},
				ReturnType: typesystem.Nil,
			},
			"mockHttpError": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, stringType},
				ReturnType: typesystem.Nil,
			},
			"mockHttpOff": typesystem.TFunc{
				Params:     []typesystem.Type{},
				ReturnType: typesystem.Nil,
			},
			"mockHttpBypass": typesystem.TFunc{
				Params:     []typesystem.Type{A},
				ReturnType: A,
			},

			// File mocks
			"mockFile": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, resultStringA},
				ReturnType: typesystem.Nil,
			},
			"mockFileOff": typesystem.TFunc{
				Params:     []typesystem.Type{},
				ReturnType: typesystem.Nil,
			},
			"mockFileBypass": typesystem.TFunc{
				Params:     []typesystem.Type{A},
				ReturnType: A,
			},

			// Env mocks
			"mockEnv": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, stringType},
				ReturnType: typesystem.Nil,
			},
			"mockEnvOff": typesystem.TFunc{
				Params:     []typesystem.Type{},
				ReturnType: typesystem.Nil,
			},
			"mockEnvBypass": typesystem.TFunc{
				Params:     []typesystem.Type{A},
				ReturnType: A,
			},
		},
	}

	RegisterVirtualPackage("lib/test", pkg)
}

// initRandPackage registers the lib/rand virtual package
func initRandPackage() {
	// Generic type variable
	typeA := typesystem.TVar{Name: "A"}

	// List<A>
	listA := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{typeA},
	}

	// Option<A>
	optionA := typesystem.TApp{
		Constructor: OptionCon,
		Args:        []typesystem.Type{typeA},
	}

	pkg := &VirtualPackage{
		Name: "rand",
		Symbols: map[string]typesystem.Type{
			// randomInt: () -> Int
			"randomInt": typesystem.TFunc{
				Params:     []typesystem.Type{},
				ReturnType: typesystem.Int,
			},
			// randomIntRange: (Int, Int) -> Int
			"randomIntRange": typesystem.TFunc{
				Params:     []typesystem.Type{typesystem.Int, typesystem.Int},
				ReturnType: typesystem.Int,
			},
			// randomFloat: () -> Float
			"randomFloat": typesystem.TFunc{
				Params:     []typesystem.Type{},
				ReturnType: typesystem.Float,
			},
			// randomFloatRange: (Float, Float) -> Float
			"randomFloatRange": typesystem.TFunc{
				Params:     []typesystem.Type{typesystem.Float, typesystem.Float},
				ReturnType: typesystem.Float,
			},
			// randomBool: () -> Bool
			"randomBool": typesystem.TFunc{
				Params:     []typesystem.Type{},
				ReturnType: typesystem.Bool,
			},
			// randomChoice: List<A> -> Option<A>
			"randomChoice": typesystem.TFunc{
				Params:     []typesystem.Type{listA},
				ReturnType: optionA,
			},
			// randomShuffle: List<A> -> List<A>
			"randomShuffle": typesystem.TFunc{
				Params:     []typesystem.Type{listA},
				ReturnType: listA,
			},
			// randomSample: (List<A>, Int) -> List<A>
			"randomSample": typesystem.TFunc{
				Params:     []typesystem.Type{listA, typesystem.Int},
				ReturnType: listA,
			},
			// randomSeed: Int -> Nil
			"randomSeed": typesystem.TFunc{
				Params:     []typesystem.Type{typesystem.Int},
				ReturnType: typesystem.Nil,
			},
		},
	}

	RegisterVirtualPackage("lib/rand", pkg)
}

// initWsPackage registers WebSocket package
func initWsPackage() {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{typesystem.Char},
	}

	// Result<E, A> types - E is error, A is success
	resultInt := typesystem.TApp{
		Constructor: ResultCon,
		Args:        []typesystem.Type{stringType, typesystem.Int},
	}
	resultNil := typesystem.TApp{
		Constructor: ResultCon,
		Args:        []typesystem.Type{stringType, typesystem.Nil},
	}
	resultString := typesystem.TApp{
		Constructor: ResultCon,
		Args:        []typesystem.Type{stringType, stringType},
	}
	optionString := typesystem.TApp{
		Constructor: OptionCon,
		Args:        []typesystem.Type{stringType},
	}
	// Result<String, Option<String>> - error is String, success is Option<String>
	resultStringOptionString := typesystem.TApp{
		Constructor: ResultCon,
		Args:        []typesystem.Type{stringType, optionString},
	}

	// Handler type: (Int, String) -> String
	handlerType := typesystem.TFunc{
		Params:     []typesystem.Type{typesystem.Int, stringType},
		ReturnType: stringType,
	}

	pkg := &VirtualPackage{
		Name: "ws",
		Symbols: map[string]typesystem.Type{
			"wsConnect":        typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultInt},
			"wsConnectTimeout": typesystem.TFunc{Params: []typesystem.Type{stringType, typesystem.Int}, ReturnType: resultInt},
			"wsSend":           typesystem.TFunc{Params: []typesystem.Type{typesystem.Int, stringType}, ReturnType: resultNil},
			"wsRecv":           typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: resultString},
			"wsRecvTimeout":    typesystem.TFunc{Params: []typesystem.Type{typesystem.Int, typesystem.Int}, ReturnType: resultStringOptionString},
			"wsClose":          typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: resultNil},
			"wsServe":          typesystem.TFunc{Params: []typesystem.Type{typesystem.Int, handlerType}, ReturnType: resultNil},
			"wsServeAsync":     typesystem.TFunc{Params: []typesystem.Type{typesystem.Int, handlerType}, ReturnType: resultInt},
			"wsServerStop":     typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: resultNil},
		},
	}

	RegisterVirtualPackage("lib/ws", pkg)
}

func initSqlPackage() {
	stringType := typesystem.TApp{Constructor: ListCon, Args: []typesystem.Type{typesystem.Char}}
	nilType := typesystem.Nil
	intType := typesystem.Int
	boolType := typesystem.Bool

	// SqlDB and SqlTx are opaque types
	sqlDBType := typesystem.TCon{Name: "SqlDB"}
	sqlTxType := typesystem.TCon{Name: "SqlTx"}

	// SqlValue ADT
	sqlValueType := typesystem.TCon{Name: "SqlValue"}

	// Result types
	resultDB := typesystem.TApp{Constructor: ResultCon, Args: []typesystem.Type{stringType, sqlDBType}}
	resultTx := typesystem.TApp{Constructor: ResultCon, Args: []typesystem.Type{stringType, sqlTxType}}
	resultNil := typesystem.TApp{Constructor: ResultCon, Args: []typesystem.Type{stringType, nilType}}
	resultInt := typesystem.TApp{Constructor: ResultCon, Args: []typesystem.Type{stringType, intType}}

	// Row = Map<String, SqlValue>
	rowType := typesystem.TApp{Constructor: MapCon, Args: []typesystem.Type{stringType, sqlValueType}}
	listRow := typesystem.TApp{Constructor: ListCon, Args: []typesystem.Type{rowType}}
	optionRow := typesystem.TApp{Constructor: OptionCon, Args: []typesystem.Type{rowType}}
	resultListRow := typesystem.TApp{Constructor: ResultCon, Args: []typesystem.Type{stringType, listRow}}
	resultOptionRow := typesystem.TApp{Constructor: ResultCon, Args: []typesystem.Type{stringType, optionRow}}

	// Params = List<any> (we use a generic param type)
	anyType := typesystem.TVar{Name: "a"}
	paramsType := typesystem.TApp{Constructor: ListCon, Args: []typesystem.Type{anyType}}

	// Option<SqlValue>
	optionSqlValue := typesystem.TApp{Constructor: OptionCon, Args: []typesystem.Type{sqlValueType}}

	// Date type for SqlTime - reuse from lib/date
	var dateType typesystem.Type
	if datePkg := GetVirtualPackage("lib/date"); datePkg != nil {
		if t, ok := datePkg.Types["Date"]; ok {
			dateType = t
		}
	}

	if dateType == nil {
		// Should not happen as lib/date is initialized before lib/sql
		panic("lib/sql depends on lib/date, but Date type was not found")
	}

	bytesType := typesystem.TCon{Name: "Bytes"}
	bigIntType := typesystem.TCon{Name: "BigInt"}

	pkg := &VirtualPackage{
		Name: "sql",
		Types: map[string]typesystem.Type{
			"SqlValue": sqlValueType,
			"SqlDB":    sqlDBType,
			"SqlTx":    sqlTxType,
			// Date is NOT exported here, user must import lib/date
		},
		Constructors: map[string]typesystem.Type{
			"SqlNull":   sqlValueType,
			"SqlInt":    typesystem.TFunc{Params: []typesystem.Type{intType}, ReturnType: sqlValueType},
			"SqlFloat":  typesystem.TFunc{Params: []typesystem.Type{typesystem.Float}, ReturnType: sqlValueType},
			"SqlString": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: sqlValueType},
			"SqlBool":   typesystem.TFunc{Params: []typesystem.Type{boolType}, ReturnType: sqlValueType},
			"SqlBytes":  typesystem.TFunc{Params: []typesystem.Type{bytesType}, ReturnType: sqlValueType},
			"SqlTime":   typesystem.TFunc{Params: []typesystem.Type{dateType}, ReturnType: sqlValueType},
			"SqlBigInt": typesystem.TFunc{Params: []typesystem.Type{bigIntType}, ReturnType: sqlValueType},
		},
		Variants: map[string][]string{
			"SqlValue": {"SqlNull", "SqlInt", "SqlFloat", "SqlString", "SqlBool", "SqlBytes", "SqlTime", "SqlBigInt"},
		},
		Symbols: map[string]typesystem.Type{
			// Connection
			"sqlOpen":  typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: resultDB},
			"sqlClose": typesystem.TFunc{Params: []typesystem.Type{sqlDBType}, ReturnType: resultNil},
			"sqlPing":  typesystem.TFunc{Params: []typesystem.Type{sqlDBType}, ReturnType: resultNil},

			// Query
			"sqlQuery":        typesystem.TFunc{Params: []typesystem.Type{sqlDBType, stringType, paramsType}, ReturnType: resultListRow},
			"sqlQueryRow":     typesystem.TFunc{Params: []typesystem.Type{sqlDBType, stringType, paramsType}, ReturnType: resultOptionRow},
			"sqlExec":         typesystem.TFunc{Params: []typesystem.Type{sqlDBType, stringType, paramsType}, ReturnType: resultInt},
			"sqlLastInsertId": typesystem.TFunc{Params: []typesystem.Type{sqlDBType, stringType, paramsType}, ReturnType: resultInt},

			// Transaction
			"sqlBegin":    typesystem.TFunc{Params: []typesystem.Type{sqlDBType}, ReturnType: resultTx},
			"sqlCommit":   typesystem.TFunc{Params: []typesystem.Type{sqlTxType}, ReturnType: resultNil},
			"sqlRollback": typesystem.TFunc{Params: []typesystem.Type{sqlTxType}, ReturnType: resultNil},
			"sqlTxQuery":  typesystem.TFunc{Params: []typesystem.Type{sqlTxType, stringType, paramsType}, ReturnType: resultListRow},
			"sqlTxExec":   typesystem.TFunc{Params: []typesystem.Type{sqlTxType, stringType, paramsType}, ReturnType: resultInt},

			// Utility
			"sqlUnwrap": typesystem.TFunc{Params: []typesystem.Type{sqlValueType}, ReturnType: optionSqlValue},
			"sqlIsNull": typesystem.TFunc{Params: []typesystem.Type{sqlValueType}, ReturnType: boolType},
		},
	}

	RegisterVirtualPackage("lib/sql", pkg)
}

// initUrlPackage registers the lib/url virtual package
func initUrlPackage() {
	stringType := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{typesystem.Char},
	}

	optionInt := typesystem.TApp{Constructor: OptionCon, Args: []typesystem.Type{typesystem.Int}}

	// Url record type
	urlType := typesystem.TRecord{
		Fields: map[string]typesystem.Type{
			"scheme":   stringType,
			"userinfo": stringType,
			"host":     stringType,
			"port":     optionInt,
			"path":     stringType,
			"query":    stringType,
			"fragment": stringType,
		},
	}

	// Result<String, Url>
	resultUrl := typesystem.TApp{
		Constructor: ResultCon,
		Args:        []typesystem.Type{stringType, urlType},
	}

	// Option<String>
	optionString := typesystem.TApp{
		Constructor: OptionCon,
		Args:        []typesystem.Type{stringType},
	}

	// List<String>
	listString := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{stringType},
	}

	// Map<String, List<String>>
	mapStringListString := typesystem.TApp{
		Constructor: MapCon,
		Args:        []typesystem.Type{stringType, listString},
	}

	// Result<String, String>
	resultString := typesystem.TApp{
		Constructor: ResultCon,
		Args:        []typesystem.Type{stringType, stringType},
	}

	pkg := &VirtualPackage{
		Name: "url",
		Symbols: map[string]typesystem.Type{
			// Parsing
			"urlParse":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultUrl},
			"urlToString": typesystem.TFunc{Params: []typesystem.Type{urlType}, ReturnType: stringType},

			// Accessors
			"urlScheme":   typesystem.TFunc{Params: []typesystem.Type{urlType}, ReturnType: stringType},
			"urlUserinfo": typesystem.TFunc{Params: []typesystem.Type{urlType}, ReturnType: stringType},
			"urlHost":     typesystem.TFunc{Params: []typesystem.Type{urlType}, ReturnType: stringType},
			"urlPort":     typesystem.TFunc{Params: []typesystem.Type{urlType}, ReturnType: optionInt},
			"urlPath":     typesystem.TFunc{Params: []typesystem.Type{urlType}, ReturnType: stringType},
			"urlQuery":    typesystem.TFunc{Params: []typesystem.Type{urlType}, ReturnType: stringType},
			"urlFragment": typesystem.TFunc{Params: []typesystem.Type{urlType}, ReturnType: stringType},

			// Query params
			"urlQueryParams":   typesystem.TFunc{Params: []typesystem.Type{urlType}, ReturnType: mapStringListString},
			"urlQueryParam":    typesystem.TFunc{Params: []typesystem.Type{urlType, stringType}, ReturnType: optionString},
			"urlQueryParamAll": typesystem.TFunc{Params: []typesystem.Type{urlType, stringType}, ReturnType: listString},

			// Modifiers
			"urlWithScheme":    typesystem.TFunc{Params: []typesystem.Type{urlType, stringType}, ReturnType: urlType},
			"urlWithUserinfo":  typesystem.TFunc{Params: []typesystem.Type{urlType, stringType}, ReturnType: urlType},
			"urlWithHost":      typesystem.TFunc{Params: []typesystem.Type{urlType, stringType}, ReturnType: urlType},
			"urlWithPort":      typesystem.TFunc{Params: []typesystem.Type{urlType, typesystem.Int}, ReturnType: urlType},
			"urlWithPath":      typesystem.TFunc{Params: []typesystem.Type{urlType, stringType}, ReturnType: urlType},
			"urlWithQuery":     typesystem.TFunc{Params: []typesystem.Type{urlType, stringType}, ReturnType: urlType},
			"urlWithFragment":  typesystem.TFunc{Params: []typesystem.Type{urlType, stringType}, ReturnType: urlType},
			"urlAddQueryParam": typesystem.TFunc{Params: []typesystem.Type{urlType, stringType, stringType}, ReturnType: urlType},

			// Utility
			"urlJoin":   typesystem.TFunc{Params: []typesystem.Type{urlType, stringType}, ReturnType: resultUrl},
			"urlEncode": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			"urlDecode": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultString},
		},
	}

	RegisterVirtualPackage("lib/url", pkg)
}

// initPathPackage registers the lib/path virtual package
func initPathPackage() {
	stringType := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{typesystem.Char},
	}

	listString := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{stringType},
	}

	resultString := typesystem.TApp{
		Constructor: ResultCon,
		Args:        []typesystem.Type{stringType, stringType},
	}

	resultBool := typesystem.TApp{
		Constructor: ResultCon,
		Args:        []typesystem.Type{stringType, typesystem.Bool},
	}

	pkg := &VirtualPackage{
		Name: "path",
		Symbols: map[string]typesystem.Type{
			// Parsing
			"pathJoin":  typesystem.TFunc{Params: []typesystem.Type{listString}, ReturnType: stringType},
			"pathSplit": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: listString},
			"pathDir":   typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			"pathBase":  typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			"pathExt":   typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			"pathStem":  typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},

			// Manipulation
			"pathWithExt":  typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: stringType},
			"pathWithBase": typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: stringType},

			// Query
			"pathIsAbs": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: typesystem.Bool},
			"pathIsRel": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: typesystem.Bool},

			// Normalization
			"pathClean": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			"pathAbs":   typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultString},
			"pathRel":   typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: resultString},

			// Matching
			"pathMatch": typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: resultBool},

			// Separator
			"pathSep": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: stringType},

			// Temp directory
			"pathTemp": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: stringType},

			// POSIX-style (handles dotfiles correctly)
			"pathExtPosix":  typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			"pathStemPosix": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			"pathIsHidden":  typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: typesystem.Bool},
		},
	}

	RegisterVirtualPackage("lib/path", pkg)
}

// initLogPackage registers the lib/log virtual package
func initLogPackage() {
	stringType := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{typesystem.Char},
	}
	loggerType := typesystem.TCon{Name: "Logger"}
	nilType := typesystem.Nil
	boolType := typesystem.Bool

	// Map<String, a> for fields
	mapStringAny := typesystem.TApp{
		Constructor: MapCon,
		Args:        []typesystem.Type{stringType, typesystem.TVar{Name: "a"}},
	}

	resultNil := typesystem.TApp{
		Constructor: ResultCon,
		Args:        []typesystem.Type{stringType, nilType},
	}

	pkg := &VirtualPackage{
		Name: "log",
		Types: map[string]typesystem.Type{
			"Logger": loggerType,
		},
		Symbols: map[string]typesystem.Type{
			// Basic logging
			"logDebug": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: nilType},
			"logInfo":  typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: nilType},
			"logWarn":  typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: nilType},
			"logError": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: nilType},
			"logFatal": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: nilType},

			// Fatal with exit
			"logFatalExit": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: nilType},

			// Configuration
			"logLevel":  typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: nilType},
			"logFormat": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: nilType},
			"logOutput": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultNil},
			"logColor":  typesystem.TFunc{Params: []typesystem.Type{boolType}, ReturnType: nilType},

			// Structured logging
			"logWithFields": typesystem.TFunc{Params: []typesystem.Type{stringType, stringType, mapStringAny}, ReturnType: nilType},

			// Prefixed logger
			"logWithPrefix": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: loggerType},

			// Logger methods
			"loggerDebug":      typesystem.TFunc{Params: []typesystem.Type{loggerType, stringType}, ReturnType: nilType},
			"loggerInfo":       typesystem.TFunc{Params: []typesystem.Type{loggerType, stringType}, ReturnType: nilType},
			"loggerWarn":       typesystem.TFunc{Params: []typesystem.Type{loggerType, stringType}, ReturnType: nilType},
			"loggerError":      typesystem.TFunc{Params: []typesystem.Type{loggerType, stringType}, ReturnType: nilType},
			"loggerFatal":      typesystem.TFunc{Params: []typesystem.Type{loggerType, stringType}, ReturnType: nilType},
			"loggerFatalExit":  typesystem.TFunc{Params: []typesystem.Type{loggerType, stringType}, ReturnType: nilType},
			"loggerWithFields": typesystem.TFunc{Params: []typesystem.Type{loggerType, stringType, stringType, mapStringAny}, ReturnType: nilType},
		},
	}

	RegisterVirtualPackage("lib/log", pkg)
}

// initTaskPackage registers the lib/task virtual package
func initTaskPackage() {
	T := typesystem.TVar{Name: "T"}
	U := typesystem.TVar{Name: "U"}

	// Define TCons with correct Kinds to satisfy KindCheck
	taskCon := typesystem.TCon{Name: "Task", KindVal: typesystem.KArrow{Left: typesystem.Star, Right: typesystem.Star}}

	taskT := typesystem.TApp{
		Constructor: taskCon,
		Args:        []typesystem.Type{T},
	}

	taskU := typesystem.TApp{
		Constructor: taskCon,
		Args:        []typesystem.Type{U},
	}

	listTaskT := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{taskT},
	}

	listT := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{T},
	}

	stringType := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{typesystem.Char},
	}

	resultStringT := typesystem.TApp{
		Constructor: ResultCon,
		Args:        []typesystem.Type{stringType, T},
	}

	resultStringListT := typesystem.TApp{
		Constructor: ResultCon,
		Args:        []typesystem.Type{stringType, listT},
	}

	fnVoidT := typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: T}
	fnTU := typesystem.TFunc{Params: []typesystem.Type{T}, ReturnType: U}
	fnTTaskU := typesystem.TFunc{Params: []typesystem.Type{T}, ReturnType: taskU}
	fnStringT := typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: T}

	taskType := taskCon

	pkg := &VirtualPackage{
		Name: "task",
		Types: map[string]typesystem.Type{
			"Task": taskType,
		},
		Symbols: map[string]typesystem.Type{
			// Creation
			"async":       typesystem.TFunc{Params: []typesystem.Type{fnVoidT}, ReturnType: taskT},
			"taskResolve": typesystem.TFunc{Params: []typesystem.Type{T}, ReturnType: taskT},
			"taskReject":  typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: taskT},

			// Awaiting
			"await":             typesystem.TFunc{Params: []typesystem.Type{taskT}, ReturnType: resultStringT},
			"awaitTimeout":      typesystem.TFunc{Params: []typesystem.Type{taskT, typesystem.Int}, ReturnType: resultStringT},
			"awaitAll":          typesystem.TFunc{Params: []typesystem.Type{listTaskT}, ReturnType: resultStringListT},
			"awaitAllTimeout":   typesystem.TFunc{Params: []typesystem.Type{listTaskT, typesystem.Int}, ReturnType: resultStringListT},
			"awaitAny":          typesystem.TFunc{Params: []typesystem.Type{listTaskT}, ReturnType: resultStringT},
			"awaitAnyTimeout":   typesystem.TFunc{Params: []typesystem.Type{listTaskT, typesystem.Int}, ReturnType: resultStringT},
			"awaitFirst":        typesystem.TFunc{Params: []typesystem.Type{listTaskT}, ReturnType: resultStringT},
			"awaitFirstTimeout": typesystem.TFunc{Params: []typesystem.Type{listTaskT, typesystem.Int}, ReturnType: resultStringT},

			// Control
			"taskCancel":      typesystem.TFunc{Params: []typesystem.Type{taskT}, ReturnType: typesystem.Nil},
			"taskIsDone":      typesystem.TFunc{Params: []typesystem.Type{taskT}, ReturnType: typesystem.Bool},
			"taskIsCancelled": typesystem.TFunc{Params: []typesystem.Type{taskT}, ReturnType: typesystem.Bool},

			// Pool
			"taskSetGlobalPool": typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: typesystem.Nil},
			"taskGetGlobalPool": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: typesystem.Int},

			// Combinators
			"taskMap":     typesystem.TFunc{Params: []typesystem.Type{taskT, fnTU}, ReturnType: taskU},
			"taskFlatMap": typesystem.TFunc{Params: []typesystem.Type{taskT, fnTTaskU}, ReturnType: taskU},
			"taskCatch":   typesystem.TFunc{Params: []typesystem.Type{taskT, fnStringT}, ReturnType: taskT},
		},
	}

	RegisterVirtualPackage("lib/task", pkg)
}

// initFlagPackage registers the lib/flag virtual package
func initFlagPackage() {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{typesystem.Char},
	}
	// List<String>
	listString := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{stringType},
	}

	// Generic T
	T := typesystem.TVar{Name: "T"}

	pkg := &VirtualPackage{
		Name: "flag",
		Symbols: map[string]typesystem.Type{
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
		},
	}

	RegisterVirtualPackage("lib/flag", pkg)
}
