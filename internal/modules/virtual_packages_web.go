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

	// Result<String, HttpResponse> - error is String, success is HttpResponse
	resultStringResponse := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, responseType},
	}

	pkg := &VirtualPackage{
		Name: "http",
		Symbols: map[string]typesystem.Type{
			// Simple GET request
			"httpGet": typesystem.TFunc{
				Params:     []typesystem.Type{stringType},
				ReturnType: resultStringResponse,
			},

			// POST with string body
			"httpPost": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, stringType},
				ReturnType: resultStringResponse,
			},

			// POST with JSON body (auto-encodes)
			"httpPostJson": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, typesystem.TVar{Name: "A"}},
				ReturnType: resultStringResponse,
			},

			// PUT with string body
			"httpPut": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, stringType},
				ReturnType: resultStringResponse,
			},

			// DELETE request
			"httpDelete": typesystem.TFunc{
				Params:     []typesystem.Type{stringType},
				ReturnType: resultStringResponse,
			},

			// Full control request (timeout in ms, 0 = use global default)
			// Last 2 params have defaults: body="" and timeout=0
			"httpRequest": typesystem.TFunc{
				Params:       []typesystem.Type{stringType, stringType, headersType, stringType, typesystem.Int},
				ReturnType:   resultStringResponse,
				DefaultCount: 2,
			},

			// Set default timeout (milliseconds)
			"httpSetTimeout": typesystem.TFunc{
				Params:     []typesystem.Type{typesystem.Int},
				ReturnType: typesystem.Nil,
			},

			// ========== Server functions ==========

			// HttpRequest = { method: String, path: String, query: String, headers: List<(String, String)>, body: String }
			// httpServe: (Int, (HttpRequest) -> HttpResponse) -> Result<Nil, String>
			// Starts server and blocks, calling handler for each request
			"httpServe": typesystem.TFunc{
				Params: []typesystem.Type{
					typesystem.Int,
					typesystem.TFunc{
						Params: []typesystem.Type{
							// HttpRequest record
							typesystem.TRecord{
								Fields: map[string]typesystem.Type{
									"method":  stringType,
									"path":    stringType,
									"query":   stringType,
									"headers": headersType,
									"body":    stringType,
								},
							},
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
							typesystem.TRecord{
								Fields: map[string]typesystem.Type{
									"method":  stringType,
									"path":    stringType,
									"query":   stringType,
									"headers": headersType,
									"body":    stringType,
								},
							},
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
	}

	RegisterVirtualPackage("lib/http", pkg)
}

// initTestPackage registers the lib/test virtual package
