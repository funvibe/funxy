package evaluator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/funvibe/funxy/internal/typesystem"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// HTTP client timeout (default 30 seconds)
var httpTimeout = 30 * time.Second

// Default server stop timeout
const DefaultServerStopTimeoutMs = 5000

// Running HTTP servers (for async mode)
var (
	httpServers       = make(map[int64]*http.Server)
	httpServersMu     sync.Mutex
	httpServerCounter int64 = 0
)

// HttpBuiltins returns built-in functions for lib/http virtual package
func HttpBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		"httpGet":        {Fn: builtinHttpGet, Name: "httpGet"},
		"httpPost":       {Fn: builtinHttpPost, Name: "httpPost"},
		"httpPostJson":   {Fn: builtinHttpPostJson, Name: "httpPostJson"},
		"httpPut":        {Fn: builtinHttpPut, Name: "httpPut"},
		"httpDelete":     {Fn: builtinHttpDelete, Name: "httpDelete"},
		"httpRequest":    {Fn: builtinHttpRequest, Name: "httpRequest"},
		"httpSetTimeout": {Fn: builtinHttpSetTimeout, Name: "httpSetTimeout"},
		"httpServe":      {Fn: builtinHttpServe, Name: "httpServe"},
		"httpServeAsync": {Fn: builtinHttpServeAsync, Name: "httpServeAsync"},
		"httpServerStop": {Fn: builtinHttpServerStop, Name: "httpServerStop"},
	}
}

// getBodyReader converts String or Bytes object to io.Reader
func getBodyReader(obj Object) (io.Reader, error) {
	switch o := obj.(type) {
	case *List:
		// Empty list is empty string
		if o.len() == 0 || isStringList(o) {
			return strings.NewReader(ListToString(o)), nil
		}
		return nil, fmt.Errorf("expected String or Bytes body, got %s", o.Type())
	case *Bytes:
		return bytes.NewReader(o.data), nil
	default:
		return nil, fmt.Errorf("expected String or Bytes body, got %s", o.Type())
	}
}

// httpGet: (String) -> Result<String, HttpResponse>
func builtinHttpGet(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("httpGet expects 1 argument, got %d", len(args))
	}

	urlList, ok := args[0].(*List)
	if !ok {
		return newError("httpGet expects a string URL, got %s", args[0].Type())
	}

	url := ListToString(urlList)
	return doHttpRequest("GET", url, nil, nil)
}

// httpPost: (String, String | Bytes) -> Result<String, HttpResponse>
func builtinHttpPost(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("httpPost expects 2 arguments, got %d", len(args))
	}

	urlList, ok := args[0].(*List)
	if !ok {
		return newError("httpPost expects a string URL, got %s", args[0].Type())
	}

	bodyReader, err := getBodyReader(args[1])
	if err != nil {
		return newError("httpPost: %s", err.Error())
	}

	url := ListToString(urlList)
	return doHttpRequest("POST", url, nil, bodyReader)
}

// httpPostJson: (String, A) -> Result<String, HttpResponse>
func builtinHttpPostJson(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("httpPostJson expects 2 arguments, got %d", len(args))
	}

	urlList, ok := args[0].(*List)
	if !ok {
		return newError("httpPostJson expects a string URL, got %s", args[0].Type())
	}

	url := ListToString(urlList)

	// Encode data to JSON
	jsonBody, err := objectToJson(args[1])
	if err != nil {
		return makeFail(stringToList("failed to encode JSON: " + err.Error()))
	}

	headers := [][2]string{{"Content-Type", "application/json"}}
	return doHttpRequest("POST", url, headers, strings.NewReader(jsonBody))
}

// httpPut: (String, String | Bytes) -> Result<String, HttpResponse>
func builtinHttpPut(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("httpPut expects 2 arguments, got %d", len(args))
	}

	urlList, ok := args[0].(*List)
	if !ok {
		return newError("httpPut expects a string URL, got %s", args[0].Type())
	}

	bodyReader, err := getBodyReader(args[1])
	if err != nil {
		return newError("httpPut: %s", err.Error())
	}

	url := ListToString(urlList)
	return doHttpRequest("PUT", url, nil, bodyReader)
}

// httpDelete: (String) -> Result<String, HttpResponse>
func builtinHttpDelete(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("httpDelete expects 1 argument, got %d", len(args))
	}

	urlList, ok := args[0].(*List)
	if !ok {
		return newError("httpDelete expects a string URL, got %s", args[0].Type())
	}

	url := ListToString(urlList)
	return doHttpRequest("DELETE", url, nil, nil)
}

// httpRequest: (String, String, List<(String, String)>, String, Int) -> Result<String, HttpResponse>
// timeout is in milliseconds, 0 or negative means use global default
func builtinHttpRequest(e *Evaluator, args ...Object) Object {
	if len(args) != 5 {
		return newError("httpRequest expects 5 arguments, got %d", len(args))
	}

	methodList, ok := args[0].(*List)
	if !ok {
		return newError("httpRequest expects a string method, got %s", args[0].Type())
	}

	urlList, ok := args[1].(*List)
	if !ok {
		return newError("httpRequest expects a string URL, got %s", args[1].Type())
	}

	headersList, ok := args[2].(*List)
	if !ok {
		return newError("httpRequest expects a list of headers, got %s", args[2].Type())
	}

	bodyReader, err := getBodyReader(args[3])
	if err != nil {
		return newError("httpRequest: %s", err.Error())
	}

	timeoutInt, ok := args[4].(*Integer)
	if !ok {
		return newError("httpRequest expects an integer timeout (ms), got %s", args[4].Type())
	}

	method := ListToString(methodList)
	url := ListToString(urlList)

	// Parse headers
	var headers [][2]string
	for _, h := range headersList.ToSlice() {
		tuple, ok := h.(*Tuple)
		if !ok || len(tuple.Elements) != 2 {
			return newError("httpRequest expects headers as list of (String, String) tuples")
		}
		keyList, ok1 := tuple.Elements[0].(*List)
		valList, ok2 := tuple.Elements[1].(*List)
		if !ok1 || !ok2 {
			return newError("httpRequest header key and value must be strings")
		}
		headers = append(headers, [2]string{ListToString(keyList), ListToString(valList)})
	}

	// Use per-request timeout if specified, otherwise global
	timeout := httpTimeout
	if timeoutInt.Value > 0 {
		timeout = time.Duration(timeoutInt.Value) * time.Millisecond
	}

	return doHttpRequestWithTimeout(method, url, headers, bodyReader, timeout)
}

// httpSetTimeout: (Int) -> Nil
func builtinHttpSetTimeout(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("httpSetTimeout expects 1 argument, got %d", len(args))
	}

	msInt, ok := args[0].(*Integer)
	if !ok {
		return newError("httpSetTimeout expects an integer (milliseconds), got %s", args[0].Type())
	}

	httpTimeout = time.Duration(msInt.Value) * time.Millisecond
	return &Nil{}
}

// doHttpRequest performs the actual HTTP request with global timeout
func doHttpRequest(method, url string, headers [][2]string, body io.Reader) Object {
	return doHttpRequestWithTimeout(method, url, headers, body, httpTimeout)
}

// doHttpRequestWithTimeout performs HTTP request with specified timeout
func doHttpRequestWithTimeout(method, url string, headers [][2]string, body io.Reader, timeout time.Duration) Object {
	// Check for HTTP mocks first
	tr := GetTestRunner()

	// Check for error mock
	if errMsg, found := tr.FindHttpMockError(url); found {
		return makeFail(stringToList(errMsg))
	}

	// Check for response mock
	if mockResp, found := tr.FindHttpMock(url); found {
		return makeOk(mockResp)
	}

	// Check if we should block real HTTP (mocks active but no match)
	if tr.ShouldBlockHttp(url) {
		return makeFail(stringToList("HTTP request blocked: no mock found for " + url))
	}

	// Make real HTTP request
	client := &http.Client{
		Timeout: timeout,
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return makeFail(stringToList("failed to create request: " + err.Error()))
	}

	// Set headers
	for _, h := range headers {
		req.Header.Set(h[0], h[1])
	}

	resp, err := client.Do(req)
	if err != nil {
		return makeFail(stringToList("request failed: " + err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return makeFail(stringToList("failed to read response: " + err.Error()))
	}

	// Build response headers
	var respHeaders []Object
	for key, values := range resp.Header {
		for _, val := range values {
			respHeaders = append(respHeaders, &Tuple{
				Elements: []Object{stringToList(key), stringToList(val)},
			})
		}
	}

	// Build response record
	response := NewRecord(map[string]Object{
		"status":  &Integer{Value: int64(resp.StatusCode)},
		"body":    stringToList(string(respBody)),
		"headers": newList(respHeaders),
	})

	return makeOk(response)
}

// objectToJson converts an Object to JSON string
func objectToJson(obj Object) (string, error) {
	goVal := objectToGoValue(obj)
	jsonBytes, err := json.Marshal(goVal)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

// objectToGoValue converts Object to Go value for JSON encoding
func objectToGoValue(obj Object) interface{} {
	switch o := obj.(type) {
	case *Integer:
		return o.Value
	case *Float:
		return o.Value
	case *Boolean:
		return o.Value
	case *Char:
		return string(rune(o.Value))
	case *List:
		// Check if it's a string (list of chars)
		if isStringList(o) {
			return ListToString(o)
		}
		// Regular list
		arr := make([]interface{}, o.len())
		for i, el := range o.ToSlice() {
			arr[i] = objectToGoValue(el)
		}
		return arr
	case *Tuple:
		arr := make([]interface{}, len(o.Elements))
		for i, el := range o.Elements {
			arr[i] = objectToGoValue(el)
		}
		return arr
	case *RecordInstance:
		m := make(map[string]interface{})
		for _, f := range o.Fields {
			m[f.Key] = objectToGoValue(f.Value)
		}
		return m
	case *DataInstance:
		// Handle Option/Result etc
		switch o.Name {
		case "Some":
			if len(o.Fields) > 0 {
				return objectToGoValue(o.Fields[0])
			}
			return nil
		case "None", "JNull":
			return nil
		case "Ok":
			if len(o.Fields) > 0 {
				return objectToGoValue(o.Fields[0])
			}
			return nil
		case "Fail":
			if len(o.Fields) > 0 {
				return map[string]interface{}{"error": objectToGoValue(o.Fields[0])}
			}
			return map[string]interface{}{"error": nil}
		default:
			// Generic ADT - return as object with constructor
			if len(o.Fields) == 0 {
				return o.Name
			}
			if len(o.Fields) == 1 {
				return objectToGoValue(o.Fields[0])
			}
			arr := make([]interface{}, len(o.Fields))
			for i, f := range o.Fields {
				arr[i] = objectToGoValue(f)
			}
			return arr
		}
	case *Nil:
		return nil
	default:
		return nil
	}
}

// isStringList checks if a list is a string (list of chars)
func isStringList(l *List) bool {
	if l.len() == 0 {
		return false
	}
	_, ok := l.get(0).(*Char)
	return ok
}

// httpServe: (Int, (HttpRequest) -> HttpResponse) -> Result<String, Nil>
func builtinHttpServe(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("httpServe expects 2 arguments, got %d", len(args))
	}

	portInt, ok := args[0].(*Integer)
	if !ok {
		return newError("httpServe expects an integer port, got %s", args[0].Type())
	}

	handler := args[1]
	if !httpIsCallable(handler) {
		return newError("httpServe expects a handler function, got %s", args[1].Type())
	}

	port := int(portInt.Value)

	// Capture handler if CaptureHandler is available
	if e.CaptureHandler != nil {
		handler = e.CaptureHandler(handler)
	}

	// Create a snapshot of the evaluator/VM for the server
	// This avoids race conditions when the main VM continues execution and modifies globals
	var serverEval *Evaluator
	if e.Fork != nil {
		serverEval = e.Fork()
	} else {
		serverEval = e.Clone()
	}

	// Create HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Create a fresh evaluator/VM for each request from the snapshot
		var reqEval *Evaluator
		if serverEval.Fork != nil {
			reqEval = serverEval.Fork()
		} else {
			reqEval = serverEval.Clone()
		}

		// Build HttpRequest object
		var headers []Object
		for key, values := range r.Header {
			for _, val := range values {
				headers = append(headers, &Tuple{
					Elements: []Object{stringToList(key), stringToList(val)},
				})
			}
		}

		bodyBytes, _ := io.ReadAll(r.Body)
		defer func() { _ = r.Body.Close() }()

		request := NewRecord(map[string]Object{
			"method":  stringToList(r.Method),
			"path":    stringToList(r.URL.Path),
			"query":   stringToList(r.URL.RawQuery),
			"headers": newList(headers),
			"body":    stringToList(string(bodyBytes)),
		})

		// Call handler
		result := reqEval.ApplyFunction(handler, []Object{request})

		// Parse response
		if result == nil {
			w.WriteHeader(500)
			_, _ = w.Write([]byte("Handler returned nil"))
			return
		}

		if errObj, ok := result.(*Error); ok {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(errObj.Message))
			return
		}

		respRec, ok := result.(*RecordInstance)
		if !ok {
			w.WriteHeader(500)
			_, _ = w.Write([]byte("Handler must return HttpResponse record"))
			return
		}

		// Set response headers
		if headersObj := respRec.Get("headers"); headersObj != nil {
			if headersList, ok := headersObj.(*List); ok {
				for _, h := range headersList.ToSlice() {
					if tuple, ok := h.(*Tuple); ok && len(tuple.Elements) == 2 {
						key := ListToString(tuple.Elements[0].(*List))
						val := ListToString(tuple.Elements[1].(*List))
						w.Header().Set(key, val)
					}
				}
			}
		}

		// Set status
		status := 200
		if statusObj := respRec.Get("status"); statusObj != nil {
			if statusInt, ok := statusObj.(*Integer); ok {
				status = int(statusInt.Value)
			}
		}
		w.WriteHeader(status)

		// Write body
		if bodyObj := respRec.Get("body"); bodyObj != nil {
			if bodyList, ok := bodyObj.(*List); ok {
				_, _ = w.Write([]byte(ListToString(bodyList)))
			}
		}
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	// Start server (blocking)
	err := server.ListenAndServe()
	if err != nil {
		return makeFail(stringToList(err.Error()))
	}

	return makeOk(&Nil{})
}

// httpServeAsync: (Int, (HttpRequest) -> HttpResponse) -> Int
// Starts a non-blocking HTTP server and returns server ID
func builtinHttpServeAsync(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("httpServeAsync expects 2 arguments, got %d", len(args))
	}

	portInt, ok := args[0].(*Integer)
	if !ok {
		return newError("httpServeAsync expects an integer port, got %s", args[0].Type())
	}

	handler := args[1]
	// Check for tree-walk Function or VM closure
	if !httpIsCallable(handler) {
		return newError("httpServeAsync expects a handler function, got %s", args[1].Type())
	}

	port := int(portInt.Value)

	// Capture handler if CaptureHandler is available
	if e.CaptureHandler != nil {
		handler = e.CaptureHandler(handler)
	}

	// Create a snapshot of the evaluator/VM for the server
	// This avoids race conditions when the main VM continues execution and modifies globals
	var serverEval *Evaluator
	if e.Fork != nil {
		serverEval = e.Fork()
	} else {
		serverEval = e.Clone()
	}

	// Create HTTP server with same handler logic as httpServe
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Create a fresh evaluator/VM for each request from the snapshot
		var reqEval *Evaluator
		if serverEval.Fork != nil {
			reqEval = serverEval.Fork()
		} else {
			reqEval = serverEval.Clone()
		}

		// Build HttpRequest object
		var headers []Object
		for key, values := range r.Header {
			for _, val := range values {
				headers = append(headers, &Tuple{
					Elements: []Object{stringToList(key), stringToList(val)},
				})
			}
		}

		bodyBytes, _ := io.ReadAll(r.Body)
		defer func() { _ = r.Body.Close() }()

		request := NewRecord(map[string]Object{
			"method":  stringToList(r.Method),
			"path":    stringToList(r.URL.Path),
			"query":   stringToList(r.URL.RawQuery),
			"headers": newList(headers),
			"body":    stringToList(string(bodyBytes)),
		})

		// Call handler
		result := reqEval.ApplyFunction(handler, []Object{request})

		// Parse response
		if result == nil {
			w.WriteHeader(500)
			_, _ = w.Write([]byte("Handler returned nil"))
			return
		}

		if errObj, ok := result.(*Error); ok {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(errObj.Message))
			return
		}

		respRec, ok := result.(*RecordInstance)
		if !ok {
			w.WriteHeader(500)
			_, _ = w.Write([]byte("Handler must return HttpResponse record"))
			return
		}

		// Set response headers
		if headersObj := respRec.Get("headers"); headersObj != nil {
			if headersList, ok := headersObj.(*List); ok {
				for _, h := range headersList.ToSlice() {
					if tuple, ok := h.(*Tuple); ok && len(tuple.Elements) == 2 {
						key := ListToString(tuple.Elements[0].(*List))
						val := ListToString(tuple.Elements[1].(*List))
						w.Header().Set(key, val)
					}
				}
			}
		}

		// Set status
		status := 200
		if statusObj := respRec.Get("status"); statusObj != nil {
			if statusInt, ok := statusObj.(*Integer); ok {
				status = int(statusInt.Value)
			}
		}
		w.WriteHeader(status)

		// Write body
		if bodyObj := respRec.Get("body"); bodyObj != nil {
			if bodyList, ok := bodyObj.(*List); ok {
				_, _ = w.Write([]byte(ListToString(bodyList)))
			}
		}
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	// Generate server ID
	httpServersMu.Lock()
	httpServerCounter++
	serverId := httpServerCounter

	// Store server
	httpServers[serverId] = server
	httpServersMu.Unlock()

	// Start server in background (non-blocking)
	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			// Log error but don't fail - server might have been stopped
		}
		// Clean up when server stops
		httpServersMu.Lock()
		delete(httpServers, serverId)
		httpServersMu.Unlock()
	}()

	// Give server a moment to start
	time.Sleep(10 * time.Millisecond)

	return &Integer{Value: serverId}
}

// httpServerStop: (Int, Int) -> Nil
// Stops a running HTTP server by ID. Optional second argument is timeout in ms (default 5000).
func builtinHttpServerStop(e *Evaluator, args ...Object) Object {
	if len(args) < 1 || len(args) > 2 {
		return newError("httpServerStop expects 1 or 2 arguments, got %d", len(args))
	}

	idInt, ok := args[0].(*Integer)
	if !ok {
		return newError("httpServerStop expects an integer server ID, got %s", args[0].Type())
	}

	timeoutMs := DefaultServerStopTimeoutMs
	if len(args) == 2 {
		t, ok := args[1].(*Integer)
		if !ok {
			return newError("httpServerStop expects an integer timeout, got %s", args[1].Type())
		}
		timeoutMs = int(t.Value)
	}

	serverId := idInt.Value

	httpServersMu.Lock()
	server, exists := httpServers[serverId]
	if exists {
		// Remove from map immediately to prevent double-stop
		delete(httpServers, serverId)
	}
	httpServersMu.Unlock()

	if !exists {
		return newError("server with ID %d not found", serverId)
	}

	// Shutdown server gracefully
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return newError("error shutting down server: %s", err.Error())
	}

	return &Nil{}
}

// SetHttpBuiltinTypes sets type info for http builtins
func SetHttpBuiltinTypes(builtins map[string]*Builtin) {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}
	stringOrBytes := typesystem.StringOrBytes

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

	// Result<String, HttpResponse>
	resultResponse := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, responseType},
	}

	// HttpRequest type for server
	requestType := typesystem.TRecord{
		Fields: map[string]typesystem.Type{
			"method":  stringType,
			"path":    stringType,
			"query":   stringType,
			"headers": headersType,
			"body":    stringType,
		},
	}

	// Handler function type: (HttpRequest) -> HttpResponse
	handlerType := typesystem.TFunc{
		Params:     []typesystem.Type{requestType},
		ReturnType: responseType,
	}

	// Result<String, Nil>
	resultNil := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, typesystem.Nil},
	}

	types := map[string]typesystem.Type{
		"httpGet":      typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultResponse},
		"httpPost":     typesystem.TFunc{Params: []typesystem.Type{stringType, stringOrBytes}, ReturnType: resultResponse},
		"httpPostJson": typesystem.TFunc{Params: []typesystem.Type{stringType, typesystem.TVar{Name: "A"}}, ReturnType: resultResponse},
		"httpPut":      typesystem.TFunc{Params: []typesystem.Type{stringType, stringOrBytes}, ReturnType: resultResponse},
		"httpDelete":   typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultResponse},
		// httpRequest has 2 default params: body="" and timeout=0
		"httpRequest": typesystem.TFunc{
			Params:       []typesystem.Type{stringType, stringType, headersType, stringOrBytes, typesystem.Int},
			ReturnType:   resultResponse,
			DefaultCount: 2, // body and timeout have defaults
		},
		"httpSetTimeout": typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: typesystem.Nil},
		"httpServe":      typesystem.TFunc{Params: []typesystem.Type{typesystem.Int, handlerType}, ReturnType: resultNil},
		"httpServeAsync": typesystem.TFunc{Params: []typesystem.Type{typesystem.Int, handlerType}, ReturnType: typesystem.Int},
		"httpServerStop": typesystem.TFunc{
			Params:       []typesystem.Type{typesystem.Int, typesystem.Int},
			ReturnType:   typesystem.Nil,
			DefaultCount: 1, // timeout has default
		},
	}

	for name, typ := range types {
		if b, ok := builtins[name]; ok {
			b.TypeInfo = typ
		}
	}

	// Set default arguments for httpRequest: body="" and timeout=0
	if b, ok := builtins["httpRequest"]; ok {
		b.DefaultArgs = []Object{
			stringToList(""),   // body default = ""
			&Integer{Value: 0}, // timeout default = 0
		}
	}

	// Set default argument for httpServerStop: timeout=5000
	if b, ok := builtins["httpServerStop"]; ok {
		b.DefaultArgs = []Object{
			&Integer{Value: int64(DefaultServerStopTimeoutMs)},
		}
	}
}

// httpIsCallable checks if an object is callable (Function, Builtin, PartialApplication, or VM Closure)
func httpIsCallable(obj Object) bool {
	switch obj.(type) {
	case *Function, *Builtin, *PartialApplication:
		return true
	}
	// Check for VM closure by type string
	if obj.Type() == "CLOSURE" || obj.Type() == "BUILTIN_CLOSURE" {
		return true
	}
	return false
}
