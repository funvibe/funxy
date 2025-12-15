package evaluator

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/funvibe/funxy/internal/typesystem"
)

// UrlBuiltins returns all URL-related built-in functions
func UrlBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		// Parsing
		"urlParse":    {Fn: builtinUrlParse, Name: "urlParse"},
		"urlToString": {Fn: builtinUrlToString, Name: "urlToString"},

		// Accessors
		"urlScheme":   {Fn: builtinUrlScheme, Name: "urlScheme"},
		"urlUserinfo": {Fn: builtinUrlUserinfo, Name: "urlUserinfo"},
		"urlHost":     {Fn: builtinUrlHost, Name: "urlHost"},
		"urlPort":     {Fn: builtinUrlPort, Name: "urlPort"},
		"urlPath":     {Fn: builtinUrlPath, Name: "urlPath"},
		"urlQuery":    {Fn: builtinUrlQuery, Name: "urlQuery"},
		"urlFragment": {Fn: builtinUrlFragment, Name: "urlFragment"},

		// Query params
		"urlQueryParams":   {Fn: builtinUrlQueryParams, Name: "urlQueryParams"},
		"urlQueryParam":    {Fn: builtinUrlQueryParam, Name: "urlQueryParam"},
		"urlQueryParamAll": {Fn: builtinUrlQueryParamAll, Name: "urlQueryParamAll"},

		// Modifiers
		"urlWithScheme":    {Fn: builtinUrlWithScheme, Name: "urlWithScheme"},
		"urlWithUserinfo":  {Fn: builtinUrlWithUserinfo, Name: "urlWithUserinfo"},
		"urlWithHost":      {Fn: builtinUrlWithHost, Name: "urlWithHost"},
		"urlWithPort":      {Fn: builtinUrlWithPort, Name: "urlWithPort"},
		"urlWithPath":      {Fn: builtinUrlWithPath, Name: "urlWithPath"},
		"urlWithQuery":     {Fn: builtinUrlWithQuery, Name: "urlWithQuery"},
		"urlWithFragment":  {Fn: builtinUrlWithFragment, Name: "urlWithFragment"},
		"urlAddQueryParam": {Fn: builtinUrlAddQueryParam, Name: "urlAddQueryParam"},

		// Utility
		"urlJoin":   {Fn: builtinUrlJoin, Name: "urlJoin"},
		"urlEncode": {Fn: builtinUrlEncode, Name: "urlEncode"},
		"urlDecode": {Fn: builtinUrlDecode, Name: "urlDecode"},
	}
}

// Helper to create Url record
func createUrlRecord(u *url.URL) *RecordInstance {
	port := makeZero()
	if u.Port() != "" {
		if p, err := strconv.Atoi(u.Port()); err == nil {
			port = makeSome(&Integer{Value: int64(p)})
		}
	}

	userinfo := ""
	if u.User != nil {
		userinfo = u.User.String()
	}

	return NewRecord(map[string]Object{
		"scheme":   stringToList(u.Scheme),
		"userinfo": stringToList(userinfo),
		"host":     stringToList(u.Hostname()),
		"port":     port,
		"path":     stringToList(u.Path),
		"query":    stringToList(u.RawQuery),
		"fragment": stringToList(u.Fragment),
	})
}

// Helper to extract URL from record
func recordToUrl(rec *RecordInstance) (*url.URL, error) {
	scheme := listToString(rec.Get("scheme").(*List))
	userinfo := listToString(rec.Get("userinfo").(*List))
	host := listToString(rec.Get("host").(*List))
	path := listToString(rec.Get("path").(*List))
	query := listToString(rec.Get("query").(*List))
	fragment := listToString(rec.Get("fragment").(*List))

	// Get port
	portStr := ""
	if port, ok := rec.Get("port").(*DataInstance); ok && port.Name == "Some" {
		if len(port.Fields) > 0 {
			if portInt, ok := port.Fields[0].(*Integer); ok {
				portStr = strconv.FormatInt(portInt.Value, 10)
			}
		}
	}

	// Build URL
	u := &url.URL{
		Scheme:   scheme,
		Path:     path,
		RawQuery: query,
		Fragment: fragment,
	}

	// Set host with port
	if portStr != "" {
		u.Host = host + ":" + portStr
	} else {
		u.Host = host
	}

	// Set userinfo
	if userinfo != "" {
		parts := strings.SplitN(userinfo, ":", 2)
		if len(parts) == 2 {
			u.User = url.UserPassword(parts[0], parts[1])
		} else {
			u.User = url.User(parts[0])
		}
	}

	return u, nil
}

// urlParse: String -> Result<String, Url>
func builtinUrlParse(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("urlParse expects 1 argument, got %d", len(args))
	}

	str, ok := args[0].(*List)
	if !ok {
		return newError("urlParse expects a string, got %s", args[0].Type())
	}

	input := listToString(str)
	u, err := url.Parse(input)
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(createUrlRecord(u))
}

// urlToString: Url -> String
func builtinUrlToString(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("urlToString expects 1 argument, got %d", len(args))
	}

	rec, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("urlToString expects a Url record, got %s", args[0].Type())
	}

	u, err := recordToUrl(rec)
	if err != nil {
		return newError("urlToString: %s", err.Error())
	}

	return stringToList(u.String())
}

// urlScheme: Url -> String
func builtinUrlScheme(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("urlScheme expects 1 argument, got %d", len(args))
	}
	rec, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("urlScheme expects a Url record, got %s", args[0].Type())
	}
	return rec.Get("scheme")
}

// urlUserinfo: Url -> String
func builtinUrlUserinfo(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("urlUserinfo expects 1 argument, got %d", len(args))
	}
	rec, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("urlUserinfo expects a Url record, got %s", args[0].Type())
	}
	return rec.Get("userinfo")
}

// urlHost: Url -> String
func builtinUrlHost(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("urlHost expects 1 argument, got %d", len(args))
	}
	rec, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("urlHost expects a Url record, got %s", args[0].Type())
	}
	return rec.Get("host")
}

// urlPort: Url -> Option<Int>
func builtinUrlPort(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("urlPort expects 1 argument, got %d", len(args))
	}
	rec, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("urlPort expects a Url record, got %s", args[0].Type())
	}
	return rec.Get("port")
}

// urlPath: Url -> String
func builtinUrlPath(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("urlPath expects 1 argument, got %d", len(args))
	}
	rec, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("urlPath expects a Url record, got %s", args[0].Type())
	}
	return rec.Get("path")
}

// urlQuery: Url -> String
func builtinUrlQuery(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("urlQuery expects 1 argument, got %d", len(args))
	}
	rec, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("urlQuery expects a Url record, got %s", args[0].Type())
	}
	return rec.Get("query")
}

// urlFragment: Url -> String
func builtinUrlFragment(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("urlFragment expects 1 argument, got %d", len(args))
	}
	rec, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("urlFragment expects a Url record, got %s", args[0].Type())
	}
	return rec.Get("fragment")
}

// urlQueryParams: Url -> Map<String, List<String>>
func builtinUrlQueryParams(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("urlQueryParams expects 1 argument, got %d", len(args))
	}

	rec, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("urlQueryParams expects a Url record, got %s", args[0].Type())
	}

	queryStr := listToString(rec.Get("query").(*List))
	values, err := url.ParseQuery(queryStr)
	if err != nil {
		return newMap()
	}

	result := newMap()
	for key, vals := range values {
		// Convert []string to List<String>
		listVals := make([]Object, len(vals))
		for i, v := range vals {
			listVals[i] = stringToList(v)
		}
		result = result.put(stringToList(key), newList(listVals))
	}

	return result
}

// urlQueryParam: Url, String -> Option<String>
func builtinUrlQueryParam(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("urlQueryParam expects 2 arguments, got %d", len(args))
	}

	rec, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("urlQueryParam expects a Url record, got %s", args[0].Type())
	}

	keyList, ok := args[1].(*List)
	if !ok {
		return newError("urlQueryParam expects a string key, got %s", args[1].Type())
	}

	queryStr := listToString(rec.Get("query").(*List))
	values, err := url.ParseQuery(queryStr)
	if err != nil {
		return makeZero()
	}

	key := listToString(keyList)
	if vals, exists := values[key]; exists && len(vals) > 0 {
		return makeSome(stringToList(vals[0]))
	}

	return makeZero()
}

// urlQueryParamAll: Url, String -> List<String>
func builtinUrlQueryParamAll(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("urlQueryParamAll expects 2 arguments, got %d", len(args))
	}

	rec, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("urlQueryParamAll expects a Url record, got %s", args[0].Type())
	}

	keyList, ok := args[1].(*List)
	if !ok {
		return newError("urlQueryParamAll expects a string key, got %s", args[1].Type())
	}

	queryStr := listToString(rec.Get("query").(*List))
	values, err := url.ParseQuery(queryStr)
	if err != nil {
		return newList([]Object{})
	}

	key := listToString(keyList)
	if vals, exists := values[key]; exists {
		listVals := make([]Object, len(vals))
		for i, v := range vals {
			listVals[i] = stringToList(v)
		}
		return newList(listVals)
	}

	return newList([]Object{})
}

// urlWithScheme: Url, String -> Url
func builtinUrlWithScheme(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("urlWithScheme expects 2 arguments, got %d", len(args))
	}

	rec, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("urlWithScheme expects a Url record, got %s", args[0].Type())
	}

	scheme, ok := args[1].(*List)
	if !ok {
		return newError("urlWithScheme expects a string, got %s", args[1].Type())
	}

	newRec := copyRecord(rec)
	newRec.Set("scheme", scheme)
	return newRec
}

// urlWithUserinfo: Url, String -> Url
func builtinUrlWithUserinfo(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("urlWithUserinfo expects 2 arguments, got %d", len(args))
	}

	rec, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("urlWithUserinfo expects a Url record, got %s", args[0].Type())
	}

	userinfo, ok := args[1].(*List)
	if !ok {
		return newError("urlWithUserinfo expects a string, got %s", args[1].Type())
	}

	newRec := copyRecord(rec)
	newRec.Set("userinfo", userinfo)
	return newRec
}

// urlWithHost: Url, String -> Url
func builtinUrlWithHost(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("urlWithHost expects 2 arguments, got %d", len(args))
	}

	rec, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("urlWithHost expects a Url record, got %s", args[0].Type())
	}

	host, ok := args[1].(*List)
	if !ok {
		return newError("urlWithHost expects a string, got %s", args[1].Type())
	}

	newRec := copyRecord(rec)
	newRec.Set("host", host)
	return newRec
}

// urlWithPort: Url, Int -> Url
func builtinUrlWithPort(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("urlWithPort expects 2 arguments, got %d", len(args))
	}

	rec, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("urlWithPort expects a Url record, got %s", args[0].Type())
	}

	port, ok := args[1].(*Integer)
	if !ok {
		return newError("urlWithPort expects an Int, got %s", args[1].Type())
	}

	newRec := copyRecord(rec)
	newRec.Set("port", makeSome(port))
	return newRec
}

// urlWithPath: Url, String -> Url
func builtinUrlWithPath(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("urlWithPath expects 2 arguments, got %d", len(args))
	}

	rec, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("urlWithPath expects a Url record, got %s", args[0].Type())
	}

	path, ok := args[1].(*List)
	if !ok {
		return newError("urlWithPath expects a string, got %s", args[1].Type())
	}

	newRec := copyRecord(rec)
	newRec.Set("path", path)
	return newRec
}

// urlWithQuery: Url, String -> Url
func builtinUrlWithQuery(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("urlWithQuery expects 2 arguments, got %d", len(args))
	}

	rec, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("urlWithQuery expects a Url record, got %s", args[0].Type())
	}

	query, ok := args[1].(*List)
	if !ok {
		return newError("urlWithQuery expects a string, got %s", args[1].Type())
	}

	newRec := copyRecord(rec)
	newRec.Set("query", query)
	return newRec
}

// urlWithFragment: Url, String -> Url
func builtinUrlWithFragment(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("urlWithFragment expects 2 arguments, got %d", len(args))
	}

	rec, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("urlWithFragment expects a Url record, got %s", args[0].Type())
	}

	fragment, ok := args[1].(*List)
	if !ok {
		return newError("urlWithFragment expects a string, got %s", args[1].Type())
	}

	newRec := copyRecord(rec)
	newRec.Set("fragment", fragment)
	return newRec
}

// urlAddQueryParam: Url, String, String -> Url
func builtinUrlAddQueryParam(e *Evaluator, args ...Object) Object {
	if len(args) != 3 {
		return newError("urlAddQueryParam expects 3 arguments, got %d", len(args))
	}

	rec, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("urlAddQueryParam expects a Url record, got %s", args[0].Type())
	}

	keyList, ok := args[1].(*List)
	if !ok {
		return newError("urlAddQueryParam expects a string key, got %s", args[1].Type())
	}

	valueList, ok := args[2].(*List)
	if !ok {
		return newError("urlAddQueryParam expects a string value, got %s", args[2].Type())
	}

	queryStr := listToString(rec.Get("query").(*List))
	values, _ := url.ParseQuery(queryStr)
	if values == nil {
		values = url.Values{}
	}

	key := listToString(keyList)
	value := listToString(valueList)
	values.Add(key, value)

	newRec := copyRecord(rec)
	newRec.Set("query", stringToList(values.Encode()))
	return newRec
}

// urlJoin: Url, String -> Result<String, Url>
func builtinUrlJoin(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("urlJoin expects 2 arguments, got %d", len(args))
	}

	rec, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("urlJoin expects a Url record, got %s", args[0].Type())
	}

	refList, ok := args[1].(*List)
	if !ok {
		return newError("urlJoin expects a string, got %s", args[1].Type())
	}

	base, err := recordToUrl(rec)
	if err != nil {
		return makeFailStr(err.Error())
	}

	ref, err := url.Parse(listToString(refList))
	if err != nil {
		return makeFailStr(err.Error())
	}

	resolved := base.ResolveReference(ref)
	return makeOk(createUrlRecord(resolved))
}

// urlEncode: String -> String
func builtinUrlEncode(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("urlEncode expects 1 argument, got %d", len(args))
	}

	str, ok := args[0].(*List)
	if !ok {
		return newError("urlEncode expects a string, got %s", args[0].Type())
	}

	input := listToString(str)
	encoded := url.QueryEscape(input)
	return stringToList(encoded)
}

// urlDecode: String -> Result<String, String>
func builtinUrlDecode(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("urlDecode expects 1 argument, got %d", len(args))
	}

	str, ok := args[0].(*List)
	if !ok {
		return newError("urlDecode expects a string, got %s", args[0].Type())
	}

	input := listToString(str)
	decoded, err := url.QueryUnescape(input)
	if err != nil {
		return makeFailStr(err.Error())
	}
	return makeOk(stringToList(decoded))
}

// Helper to copy a record
func copyRecord(rec *RecordInstance) *RecordInstance {
	newFields := make([]RecordField, len(rec.Fields))
	copy(newFields, rec.Fields)
	return &RecordInstance{Fields: newFields, TypeName: rec.TypeName}
}

// SetUrlBuiltinTypes sets up type information for URL builtins
func SetUrlBuiltinTypes(builtins map[string]*Builtin) {
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}

	optionInt := typesystem.TApp{Constructor: typesystem.TCon{Name: "Option"}, Args: []typesystem.Type{typesystem.Int}}

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
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, urlType},
	}

	// Option<String>
	optionString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Option"},
		Args:        []typesystem.Type{stringType},
	}

	// List<String>
	listString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{stringType},
	}

	// Map<String, List<String>>
	mapStringListString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Map"},
		Args:        []typesystem.Type{stringType, listString},
	}

	// Result<String, String>
	resultString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, stringType},
	}

	types := map[string]typesystem.Type{
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
	}

	for name, typ := range types {
		if b, ok := builtins[name]; ok {
			b.TypeInfo = typ
		}
	}
}
