package modules

import (
	"github.com/funvibe/funxy/internal/typesystem"
)

func initHttpPackage() {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}

	// Bytes type
	bytesType := typesystem.Bytes

	// String | Bytes
	stringOrBytes := typesystem.TUnion{
		Types: []typesystem.Type{stringType, bytesType},
	}

	// (String, String) - header tuple
	headerTuple := typesystem.TTuple{
		Elements: []typesystem.Type{stringType, stringType},
	}

	// List<(String, String)> - headers
	headersType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{headerTuple},
	}

	// HttpResponse = { status: Int, body: String, headers: List<(String, String)> }
	responseType := typesystem.TRecord{
		Fields: map[string]typesystem.Type{
			"status":  typesystem.Int,
			"body":    stringType,
			"headers": headersType,
		},
	}

	// HttpRequest = { method: String, path: String, query: String, headers: List<(String, String)>, body: String }
	requestType := typesystem.TRecord{
		Fields: map[string]typesystem.Type{
			"method":  stringType,
			"path":    stringType,
			"query":   stringType,
			"headers": headersType,
			"body":    stringType,
		},
	}

	// Result<String, HttpResponse> - error is String, success is HttpResponse
	resultResponse := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, responseType},
	}

	pkg := &VirtualPackage{
		Name: "http",
		Symbols: map[string]typesystem.Type{
			// Simple GET request
			"httpGet": typesystem.TFunc{
				Params:     []typesystem.Type{stringType},
				ReturnType: resultResponse,
			},

			// POST with string body
			"httpPost": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, stringOrBytes},
				ReturnType: resultResponse,
			},

			// POST with JSON body (auto-encodes)
			"httpPostJson": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, typesystem.TVar{Name: "A"}},
				ReturnType: resultResponse,
			},

			// PUT with string body
			"httpPut": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, stringOrBytes},
				ReturnType: resultResponse,
			},

			// DELETE request
			"httpDelete": typesystem.TFunc{
				Params:     []typesystem.Type{stringType},
				ReturnType: resultResponse,
			},

			// Full control request (timeout in ms, 0 = use global default)
			// Last 2 params have defaults: body="" and timeout=0
			"httpRequest": typesystem.TFunc{
				Params:       []typesystem.Type{stringType, stringType, headersType, stringOrBytes, typesystem.Int},
				ReturnType:   resultResponse,
				DefaultCount: 2,
			},

			// Set default timeout (milliseconds)
			"httpSetTimeout": typesystem.TFunc{
				Params:     []typesystem.Type{typesystem.Int},
				ReturnType: typesystem.Nil,
			},

			// ========== Server functions ==========

			// httpServe: (Int, (HttpRequest) -> HttpResponse) -> Result<String, Nil>
			// Starts server and blocks, calling handler for each request
			"httpServe": typesystem.TFunc{
				Params: []typesystem.Type{
					typesystem.Int,
					typesystem.TFunc{
						Params: []typesystem.Type{
							// HttpRequest record
							requestType,
						},
						ReturnType: responseType,
					},
				},
				ReturnType: typesystem.TApp{
					Constructor: typesystem.TCon{Name: "Result"},
					Args:        []typesystem.Type{stringType, typesystem.Nil},
				},
			},

			// httpServeAsync: (Int, (HttpRequest) -> HttpResponse) -> Int
			// Starts server in background, returns server ID
			"httpServeAsync": typesystem.TFunc{
				Params: []typesystem.Type{
					typesystem.Int,
					typesystem.TFunc{
						Params: []typesystem.Type{
							requestType,
						},
						ReturnType: responseType,
					},
				},
				ReturnType: typesystem.Int,
			},

			// httpServerStop: (Int, Int) -> Nil
			// Stops a running server by ID. Optional timeout.
			"httpServerStop": typesystem.TFunc{
				Params:       []typesystem.Type{typesystem.Int, typesystem.Int},
				ReturnType:   typesystem.Nil,
				DefaultCount: 1,
			},
		},
		Types: map[string]typesystem.Type{
			"HttpRequest":  requestType,
			"HttpResponse": responseType,
		},
	}

	RegisterVirtualPackage("lib/http", pkg)
}

// initTestPackage registers the lib/test virtual package
