package modules

import (
	"github.com/funvibe/funxy/internal/typesystem"
)

func initJsonPackage() {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{typesystem.Char},
	}
	// Json type
	jsonType := typesystem.TCon{Name: "Json"}
	// Option<T>
	optionType := func(t typesystem.Type) typesystem.Type {
		return typesystem.TApp{
			Constructor: OptionCon,
			Args:        []typesystem.Type{t},
		}
	}
	// Result<String, T> - error is String, success is T
	resultType := func(t typesystem.Type) typesystem.Type {
		return typesystem.TApp{
			Constructor: ResultCon,
			Args:        []typesystem.Type{stringType, t},
		}
	}
	// Generic type variable
	tVar := typesystem.TVar{Name: "T"}
	pkg := &VirtualPackage{
		Name: "json",
		Types: map[string]typesystem.Type{
			"Json": jsonType,
		},
		Constructors: map[string]typesystem.Type{
			"JNull": jsonType,
			"JBool": typesystem.TFunc{Params: []typesystem.Type{typesystem.Bool}, ReturnType: jsonType},
			"JNum":  typesystem.TFunc{Params: []typesystem.Type{typesystem.Float}, ReturnType: jsonType},
			"JStr":  typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: jsonType},
			"JArr":  typesystem.TFunc{Params: []typesystem.Type{typesystem.TApp{Constructor: ListCon, Args: []typesystem.Type{jsonType}}}, ReturnType: jsonType},
			"JObj":  typesystem.TFunc{Params: []typesystem.Type{typesystem.TApp{Constructor: ListCon, Args: []typesystem.Type{typesystem.TTuple{Elements: []typesystem.Type{stringType, jsonType}}}}}, ReturnType: jsonType},
		},
		Variants: map[string][]string{
			"Json": {"JNull", "JBool", "JNum", "JStr", "JArr", "JObj"},
		},
		Symbols: map[string]typesystem.Type{
			// jsonEncode(value) -> String
			// Encodes any value to JSON string
			"jsonEncode": typesystem.TFunc{
				Params:     []typesystem.Type{typesystem.TVar{Name: "A"}},
				ReturnType: stringType,
			},
			// jsonDecode<T>(json: String) -> Result<T, String>
			// Decodes JSON string to typed value
			"jsonDecode": typesystem.TFunc{
				Params:     []typesystem.Type{stringType},
				ReturnType: resultType(tVar),
			},
			// jsonParse(str: String) -> Result<Json, String>
			// Parses JSON string into Json ADT
			"jsonParse": typesystem.TFunc{
				Params:     []typesystem.Type{stringType},
				ReturnType: resultType(jsonType),
			},
			// jsonFromValue(value) -> Json
			// Converts any value to Json ADT
			"jsonFromValue": typesystem.TFunc{
				Params:     []typesystem.Type{typesystem.TVar{Name: "A"}},
				ReturnType: jsonType,
			},
			// jsonGet(json: Json, key: String) -> Option<Json>
			// Gets a field from a JObj
			"jsonGet": typesystem.TFunc{
				Params:     []typesystem.Type{jsonType, stringType},
				ReturnType: optionType(jsonType),
			},
			// jsonKeys(json: Json) -> List<String>
			// Gets all keys from a JObj
			"jsonKeys": typesystem.TFunc{
				Params:     []typesystem.Type{jsonType},
				ReturnType: typesystem.TApp{Constructor: ListCon, Args: []typesystem.Type{stringType}},
			},
		},
	}
	RegisterVirtualPackage("lib/json", pkg)
}

// initCryptoPackage registers the lib/crypto virtual package
func initCryptoPackage() {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{typesystem.Char},
	}
	pkg := &VirtualPackage{
		Name: "crypto",
		Symbols: map[string]typesystem.Type{
			// Base64 encoding/decoding
			"base64Encode": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			"base64Decode": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			// Hex encoding/decoding
			"hexEncode": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			"hexDecode": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			// Hash functions (return hex string)
			"md5":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			"sha1":   typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			"sha256": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			"sha512": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			// HMAC (key, message) -> hex string
			"hmacSha256": typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: stringType},
			"hmacSha512": typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: stringType},
			// Cryptographically secure random
			"cryptoRandomBytes": typesystem.TFunc{
				Params: []typesystem.Type{typesystem.Int},
				ReturnType: typesystem.TApp{
					Constructor: ListCon,
					Args:        []typesystem.Type{typesystem.Int},
				},
			},
			"cryptoRandomHex": typesystem.TFunc{
				Params:     []typesystem.Type{typesystem.Int},
				ReturnType: stringType,
			},
		},
	}
	RegisterVirtualPackage("lib/crypto", pkg)
}

// initRegexPackage registers the lib/regex virtual package
func initRegexPackage() {
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
	// Option<String>
	optionString := typesystem.TApp{
		Constructor: OptionCon,
		Args:        []typesystem.Type{stringType},
	}
	// Option<List<String>>
	optionListString := typesystem.TApp{
		Constructor: OptionCon,
		Args:        []typesystem.Type{listString},
	}
	// Result<String, T> - error is String, success is T
	resultType := func(t typesystem.Type) typesystem.Type {
		return typesystem.TApp{
			Constructor: ResultCon,
			Args:        []typesystem.Type{stringType, t},
		}
	}
	pkg := &VirtualPackage{
		Name: "regex",
		Symbols: map[string]typesystem.Type{
			// Basic matching
			"regexMatch": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, stringType},
				ReturnType: typesystem.Bool,
			},
			// Find first match
			"regexFind": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, stringType},
				ReturnType: optionString,
			},
			// Find all matches
			"regexFindAll": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, stringType},
				ReturnType: listString,
			},
			// Capture groups from first match
			"regexCapture": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, stringType},
				ReturnType: optionListString,
			},
			// Replace first match
			"regexReplace": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, stringType, stringType},
				ReturnType: stringType,
			},
			// Replace all matches
			"regexReplaceAll": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, stringType, stringType},
				ReturnType: stringType,
			},
			// Split by pattern
			"regexSplit": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, stringType},
				ReturnType: listString,
			},
			// Validate regex pattern (returns error if invalid)
			"regexValidate": typesystem.TFunc{
				Params:     []typesystem.Type{stringType},
				ReturnType: resultType(typesystem.Nil),
			},
		},
	}
	RegisterVirtualPackage("lib/regex", pkg)
}

// initDatePackage registers the lib/date virtual package
func initDatePackage() {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{typesystem.Char},
	}
	// Date = { year, month, day, hour, minute, second, offset }
	// offset is in minutes from UTC (e.g., 180 = UTC+3, -300 = UTC-5)
	dateType := typesystem.TRecord{
		Fields: map[string]typesystem.Type{
			"year":   typesystem.Int,
			"month":  typesystem.Int,
			"day":    typesystem.Int,
			"hour":   typesystem.Int,
			"minute": typesystem.Int,
			"second": typesystem.Int,
			"offset": typesystem.Int,
		},
	}
	// Option<Date>
	optionDate := typesystem.TApp{
		Constructor: OptionCon,
		Args:        []typesystem.Type{dateType},
	}
	pkg := &VirtualPackage{
		Name: "date",
		Types: map[string]typesystem.Type{
			"Date": dateType,
		},
		Symbols: map[string]typesystem.Type{
			// Creation (dateNew and dateNewTime have optional offset, default = local)
			"dateNow":           typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: dateType},
			"dateNowUtc":        typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: dateType},
			"dateFromTimestamp": typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: dateType},
			"dateNew": typesystem.TFunc{
				Params:       []typesystem.Type{typesystem.Int, typesystem.Int, typesystem.Int, typesystem.Int},
				ReturnType:   dateType,
				DefaultCount: 1, // offset is optional
			},
			"dateNewTime": typesystem.TFunc{
				Params:       []typesystem.Type{typesystem.Int, typesystem.Int, typesystem.Int, typesystem.Int, typesystem.Int, typesystem.Int, typesystem.Int},
				ReturnType:   dateType,
				DefaultCount: 1, // offset is optional
			},
			"dateToTimestamp": typesystem.TFunc{Params: []typesystem.Type{dateType}, ReturnType: typesystem.Int},
			// Timezone/Offset
			"dateToUtc":      typesystem.TFunc{Params: []typesystem.Type{dateType}, ReturnType: dateType},
			"dateToLocal":    typesystem.TFunc{Params: []typesystem.Type{dateType}, ReturnType: dateType},
			"dateOffset":     typesystem.TFunc{Params: []typesystem.Type{dateType}, ReturnType: typesystem.Int},
			"dateWithOffset": typesystem.TFunc{Params: []typesystem.Type{dateType, typesystem.Int}, ReturnType: dateType},
			// Formatting
			"dateFormat": typesystem.TFunc{Params: []typesystem.Type{dateType, stringType}, ReturnType: stringType},
			"dateParse":  typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: optionDate},
			// Components
			"dateYear":    typesystem.TFunc{Params: []typesystem.Type{dateType}, ReturnType: typesystem.Int},
			"dateMonth":   typesystem.TFunc{Params: []typesystem.Type{dateType}, ReturnType: typesystem.Int},
			"dateDay":     typesystem.TFunc{Params: []typesystem.Type{dateType}, ReturnType: typesystem.Int},
			"dateWeekday": typesystem.TFunc{Params: []typesystem.Type{dateType}, ReturnType: typesystem.Int},
			"dateHour":    typesystem.TFunc{Params: []typesystem.Type{dateType}, ReturnType: typesystem.Int},
			"dateMinute":  typesystem.TFunc{Params: []typesystem.Type{dateType}, ReturnType: typesystem.Int},
			"dateSecond":  typesystem.TFunc{Params: []typesystem.Type{dateType}, ReturnType: typesystem.Int},
			// Arithmetic
			"dateAddDays":    typesystem.TFunc{Params: []typesystem.Type{dateType, typesystem.Int}, ReturnType: dateType},
			"dateAddMonths":  typesystem.TFunc{Params: []typesystem.Type{dateType, typesystem.Int}, ReturnType: dateType},
			"dateAddYears":   typesystem.TFunc{Params: []typesystem.Type{dateType, typesystem.Int}, ReturnType: dateType},
			"dateAddHours":   typesystem.TFunc{Params: []typesystem.Type{dateType, typesystem.Int}, ReturnType: dateType},
			"dateAddMinutes": typesystem.TFunc{Params: []typesystem.Type{dateType, typesystem.Int}, ReturnType: dateType},
			"dateAddSeconds": typesystem.TFunc{Params: []typesystem.Type{dateType, typesystem.Int}, ReturnType: dateType},
			// Difference
			"dateDiffDays":    typesystem.TFunc{Params: []typesystem.Type{dateType, dateType}, ReturnType: typesystem.Int},
			"dateDiffSeconds": typesystem.TFunc{Params: []typesystem.Type{dateType, dateType}, ReturnType: typesystem.Int},
		},
	}
	RegisterVirtualPackage("lib/date", pkg)
}

// initUuidPackage registers the lib/uuid virtual package
func initUuidPackage() {
	stringType := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{typesystem.Char},
	}
	uuidType := typesystem.TCon{Name: "Uuid"}
	bytesType := typesystem.TCon{Name: "Bytes"}
	intType := typesystem.Int
	boolType := typesystem.Bool
	resultUuid := typesystem.TApp{
		Constructor: ResultCon,
		Args:        []typesystem.Type{stringType, uuidType},
	}
	pkg := &VirtualPackage{
		Name: "uuid",
		Types: map[string]typesystem.Type{
			"Uuid": uuidType,
		},
		Symbols: map[string]typesystem.Type{
			// Generation
			"uuidNew": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: uuidType},
			"uuidV4":  typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: uuidType},
			"uuidV5":  typesystem.TFunc{Params: []typesystem.Type{uuidType, stringType}, ReturnType: uuidType},
			"uuidV7":  typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: uuidType},
			"uuidNil": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: uuidType},
			"uuidMax": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: uuidType},
			// Namespaces for v5
			"uuidNamespaceDNS":  typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: uuidType},
			"uuidNamespaceURL":  typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: uuidType},
			"uuidNamespaceOID":  typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: uuidType},
			"uuidNamespaceX500": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: uuidType},
			// Parsing
			"uuidParse":     typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultUuid},
			"uuidFromBytes": typesystem.TFunc{Params: []typesystem.Type{bytesType}, ReturnType: resultUuid},
			// Conversion
			"uuidToString":        typesystem.TFunc{Params: []typesystem.Type{uuidType}, ReturnType: stringType},
			"uuidToStringCompact": typesystem.TFunc{Params: []typesystem.Type{uuidType}, ReturnType: stringType},
			"uuidToStringUrn":     typesystem.TFunc{Params: []typesystem.Type{uuidType}, ReturnType: stringType},
			"uuidToStringBraces":  typesystem.TFunc{Params: []typesystem.Type{uuidType}, ReturnType: stringType},
			"uuidToStringUpper":   typesystem.TFunc{Params: []typesystem.Type{uuidType}, ReturnType: stringType},
			"uuidToBytes":         typesystem.TFunc{Params: []typesystem.Type{uuidType}, ReturnType: bytesType},
			// Info
			"uuidVersion": typesystem.TFunc{Params: []typesystem.Type{uuidType}, ReturnType: intType},
			"uuidIsNil":   typesystem.TFunc{Params: []typesystem.Type{uuidType}, ReturnType: boolType},
		},
	}
	RegisterVirtualPackage("lib/uuid", pkg)
}

// initCsvPackage registers the lib/csv virtual package
func initCsvPackage() {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{typesystem.Char},
	}
	// Result<String, T>
	resultType := func(t typesystem.Type) typesystem.Type {
		return typesystem.TApp{
			Constructor: ResultCon,
			Args:        []typesystem.Type{stringType, t},
		}
	}
	// Generic record type (open record)
	recordType := typesystem.TRecord{Fields: map[string]typesystem.Type{}, IsOpen: true}
	// List<Record>
	listRecordType := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{recordType},
	}
	// List<List<String>>
	listStringType := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{stringType},
	}
	listListStringType := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{listStringType},
	}
	pkg := &VirtualPackage{
		Name: "csv",
		Symbols: map[string]typesystem.Type{
			"csvParse": typesystem.TFunc{
				Params:       []typesystem.Type{stringType, typesystem.Char},
				ReturnType:   resultType(listRecordType),
				DefaultCount: 1,
			},
			"csvParseRaw": typesystem.TFunc{
				Params:       []typesystem.Type{stringType, typesystem.Char},
				ReturnType:   resultType(listListStringType),
				DefaultCount: 1,
			},
			"csvRead": typesystem.TFunc{
				Params:       []typesystem.Type{stringType, typesystem.Char},
				ReturnType:   resultType(listRecordType),
				DefaultCount: 1,
			},
			"csvReadRaw": typesystem.TFunc{
				Params:       []typesystem.Type{stringType, typesystem.Char},
				ReturnType:   resultType(listListStringType),
				DefaultCount: 1,
			},
			"csvEncode": typesystem.TFunc{
				Params:       []typesystem.Type{listRecordType, typesystem.Char},
				ReturnType:   stringType,
				DefaultCount: 1,
			},
			"csvEncodeRaw": typesystem.TFunc{
				Params:       []typesystem.Type{listListStringType, typesystem.Char},
				ReturnType:   stringType,
				DefaultCount: 1,
			},
			"csvWrite": typesystem.TFunc{
				Params:       []typesystem.Type{stringType, listRecordType, typesystem.Char},
				ReturnType:   resultType(typesystem.Nil),
				DefaultCount: 1,
			},
			"csvWriteRaw": typesystem.TFunc{
				Params:       []typesystem.Type{stringType, listListStringType, typesystem.Char},
				ReturnType:   resultType(typesystem.Nil),
				DefaultCount: 1,
			},
		},
	}
	RegisterVirtualPackage("lib/csv", pkg)
}

// initFlagPackage registers the lib/flag virtual package
