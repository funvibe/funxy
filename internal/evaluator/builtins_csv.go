package evaluator

import (
	"encoding/csv"
	"fmt"
	"os"
	"github.com/funvibe/funxy/internal/typesystem"
	"strings"
)

// CSV parsing and encoding functions for lib/csv

// csvParse parses a CSV string into a list of records
// First row is treated as headers
// Optional delimiter (default: comma)
func csvParse(content string, delimiter rune) (Object, error) {
	reader := csv.NewReader(strings.NewReader(content))
	if delimiter != 0 {
		reader.Comma = delimiter
	}

	// Read all records
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("CSV parse error: %v", err)
	}

	if len(records) == 0 {
		return makeOk(newList([]Object{})), nil
	}

	// First row is headers
	headers := records[0]

	// Convert remaining rows to records
	var result []Object
	for i := 1; i < len(records); i++ {
		row := records[i]
		fields := make(map[string]Object)

		for j, header := range headers {
			if j < len(row) {
				fields[header] = stringToList(row[j])
			} else {
				fields[header] = stringToList("")
			}
		}

		result = append(result, NewRecord(fields))
	}

	return makeOk(newList(result)), nil
}

// csvParseNoHeader parses CSV without treating first row as headers
// Returns list of lists
func csvParseRaw(content string, delimiter rune) (Object, error) {
	reader := csv.NewReader(strings.NewReader(content))
	if delimiter != 0 {
		reader.Comma = delimiter
	}

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("CSV parse error: %v", err)
	}

	var result []Object
	for _, row := range records {
		var rowList []Object
		for _, cell := range row {
			rowList = append(rowList, stringToList(cell))
		}
		result = append(result, newList(rowList))
	}

	return makeOk(newList(result)), nil
}

// csvRead reads and parses a CSV file
func csvRead(path string, delimiter rune) (Object, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return makeFailStr(fmt.Sprintf("Cannot read file: %v", err)), nil
	}

	return csvParse(string(content), delimiter)
}

// csvReadRaw reads CSV file without header processing
func csvReadRaw(path string, delimiter rune) (Object, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return makeFailStr(fmt.Sprintf("Cannot read file: %v", err)), nil
	}

	return csvParseRaw(string(content), delimiter)
}

// csvEncode encodes a list of records to CSV string
func csvEncode(obj Object, delimiter rune) (string, error) {
	list, ok := obj.(*List)
	if !ok {
		return "", fmt.Errorf("csvEncode expects a list of records")
	}

	if list.len() == 0 {
		return "", nil
	}

	var buf strings.Builder
	writer := csv.NewWriter(&buf)
	if delimiter != 0 {
		writer.Comma = delimiter
	}

	// Get headers from first record
	var headers []string
	firstRow := list.get(0)
	if rec, ok := firstRow.(*RecordInstance); ok {
		for _, f := range rec.Fields {
			headers = append(headers, f.Key)
		}
	} else {
		return "", fmt.Errorf("csvEncode expects records, got %s", firstRow.Type())
	}

	// Sort headers for consistent output
	// (Go maps don't have consistent order)
	sortStrings(headers)

	// Write header row
	if err := writer.Write(headers); err != nil {
		return "", err
	}

	// Write data rows
	for _, item := range list.ToSlice() {
		rec, ok := item.(*RecordInstance)
		if !ok {
			return "", fmt.Errorf("csvEncode expects records, got %s", item.Type())
		}

		var row []string
		for _, header := range headers {
			if val := rec.Get(header); val != nil {
				row = append(row, csvObjToStr(val))
			} else {
				row = append(row, "")
			}
		}

		if err := writer.Write(row); err != nil {
			return "", err
		}
	}

	writer.Flush()
	return buf.String(), writer.Error()
}

// csvEncodeRaw encodes a list of lists to CSV string (no headers)
func csvEncodeRaw(obj Object, delimiter rune) (string, error) {
	list, ok := obj.(*List)
	if !ok {
		return "", fmt.Errorf("csvEncodeRaw expects a list of lists")
	}

	var buf strings.Builder
	writer := csv.NewWriter(&buf)
	if delimiter != 0 {
		writer.Comma = delimiter
	}

	for _, item := range list.ToSlice() {
		rowList, ok := item.(*List)
		if !ok {
			return "", fmt.Errorf("csvEncodeRaw expects list of lists, got %s", item.Type())
		}

		var row []string
		for _, cell := range rowList.ToSlice() {
			row = append(row, csvObjToStr(cell))
		}

		if err := writer.Write(row); err != nil {
			return "", err
		}
	}

	writer.Flush()
	return buf.String(), writer.Error()
}

// csvWrite writes records to a CSV file
func csvWrite(path string, obj Object, delimiter rune) (Object, error) {
	content, err := csvEncode(obj, delimiter)
	if err != nil {
		return makeFailStr(err.Error()), nil
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return makeFailStr(fmt.Sprintf("Cannot write file: %v", err)), nil
	}

	return makeOk(&Nil{}), nil
}

// csvWriteRaw writes raw data (list of lists) to CSV file
func csvWriteRaw(path string, obj Object, delimiter rune) (Object, error) {
	content, err := csvEncodeRaw(obj, delimiter)
	if err != nil {
		return makeFailStr(err.Error()), nil
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return makeFailStr(fmt.Sprintf("Cannot write file: %v", err)), nil
	}

	return makeOk(&Nil{}), nil
}

// Helper: convert object to string for CSV output
func csvObjToStr(obj Object) string {
	switch v := obj.(type) {
	case *List:
		// String (List<Char>)
		return listToString(v)
	case *Integer:
		return fmt.Sprintf("%d", v.Value)
	case *Float:
		return fmt.Sprintf("%g", v.Value)
	case *Boolean:
		if v.Value {
			return "true"
		}
		return "false"
	case *Nil:
		return ""
	default:
		return v.Inspect()
	}
}

// Helper: sort strings
func sortStrings(s []string) {
	for i := 0; i < len(s)-1; i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// CsvBuiltins returns built-in functions for lib/csv virtual package
func CsvBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		"csvParse":     {Name: "csvParse", Fn: builtinCsvParse},
		"csvParseRaw":  {Name: "csvParseRaw", Fn: builtinCsvParseRaw},
		"csvRead":      {Name: "csvRead", Fn: builtinCsvRead},
		"csvReadRaw":   {Name: "csvReadRaw", Fn: builtinCsvReadRaw},
		"csvEncode":    {Name: "csvEncode", Fn: builtinCsvEncode},
		"csvEncodeRaw": {Name: "csvEncodeRaw", Fn: builtinCsvEncodeRaw},
		"csvWrite":     {Name: "csvWrite", Fn: builtinCsvWrite},
		"csvWriteRaw":  {Name: "csvWriteRaw", Fn: builtinCsvWriteRaw},
	}
}

// SetCsvBuiltinTypes sets type info for CSV builtins
func SetCsvBuiltinTypes(builtins map[string]*Builtin) {
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}

	resultType := func(t typesystem.Type) typesystem.Type {
		return typesystem.TApp{
			Constructor: typesystem.TCon{Name: "Result"},
			Args:        []typesystem.Type{stringType, t},
		}
	}

	recordType := typesystem.TRecord{Fields: map[string]typesystem.Type{}, IsOpen: true}
	listRecordType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{recordType},
	}
	listStringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{stringType},
	}
	listListStringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{listStringType},
	}

	types := map[string]typesystem.Type{
		"csvParse": typesystem.TFunc{
			Params:     []typesystem.Type{stringType},
			ReturnType: resultType(listRecordType),
		},
		"csvParseRaw": typesystem.TFunc{
			Params:     []typesystem.Type{stringType},
			ReturnType: resultType(listListStringType),
		},
		"csvRead": typesystem.TFunc{
			Params:     []typesystem.Type{stringType},
			ReturnType: resultType(listRecordType),
		},
		"csvReadRaw": typesystem.TFunc{
			Params:     []typesystem.Type{stringType},
			ReturnType: resultType(listListStringType),
		},
		"csvEncode": typesystem.TFunc{
			Params:     []typesystem.Type{listRecordType},
			ReturnType: stringType,
		},
		"csvEncodeRaw": typesystem.TFunc{
			Params:     []typesystem.Type{listListStringType},
			ReturnType: stringType,
		},
		"csvWrite": typesystem.TFunc{
			Params:     []typesystem.Type{stringType, listRecordType},
			ReturnType: resultType(typesystem.Nil),
		},
		"csvWriteRaw": typesystem.TFunc{
			Params:     []typesystem.Type{stringType, listListStringType},
			ReturnType: resultType(typesystem.Nil),
		},
	}

	for name, typ := range types {
		if b, ok := builtins[name]; ok {
			b.TypeInfo = typ
		}
	}
}

// Builtin function implementations

// getDelimiter extracts optional delimiter from args
func getDelimiter(args []Object, idx int) rune {
	if len(args) > idx {
		if c, ok := args[idx].(*Char); ok {
			return rune(c.Value)
		}
	}
	return 0 // default comma
}

// Builtin function implementations

func builtinCsvParse(e *Evaluator, args ...Object) Object {
	if len(args) < 1 || len(args) > 2 {
		return newError("csvParse(content, delimiter?)")
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("csvParse: first argument must be String")
	}
	result, err := csvParse(listToString(list), getDelimiter(args, 1))
	if err != nil {
		return makeFailStr(err.Error())
	}
	return result
}

func builtinCsvParseRaw(e *Evaluator, args ...Object) Object {
	if len(args) < 1 || len(args) > 2 {
		return newError("csvParseRaw(content, delimiter?)")
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("csvParseRaw: first argument must be String")
	}
	result, err := csvParseRaw(listToString(list), getDelimiter(args, 1))
	if err != nil {
		return makeFailStr(err.Error())
	}
	return result
}

func builtinCsvRead(e *Evaluator, args ...Object) Object {
	if len(args) < 1 || len(args) > 2 {
		return newError("csvRead(path, delimiter?)")
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("csvRead: first argument must be String")
	}
	result, err := csvRead(listToString(list), getDelimiter(args, 1))
	if err != nil {
		return makeFailStr(err.Error())
	}
	return result
}

func builtinCsvReadRaw(e *Evaluator, args ...Object) Object {
	if len(args) < 1 || len(args) > 2 {
		return newError("csvReadRaw(path, delimiter?)")
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("csvReadRaw: first argument must be String")
	}
	result, err := csvReadRaw(listToString(list), getDelimiter(args, 1))
	if err != nil {
		return makeFailStr(err.Error())
	}
	return result
}

func builtinCsvEncode(e *Evaluator, args ...Object) Object {
	if len(args) < 1 || len(args) > 2 {
		return newError("csvEncode(records, delimiter?)")
	}
	result, err := csvEncode(args[0], getDelimiter(args, 1))
	if err != nil {
		return newError("csvEncode: %s", err.Error())
	}
	return stringToList(result)
}

func builtinCsvEncodeRaw(e *Evaluator, args ...Object) Object {
	if len(args) < 1 || len(args) > 2 {
		return newError("csvEncodeRaw(rows, delimiter?)")
	}
	result, err := csvEncodeRaw(args[0], getDelimiter(args, 1))
	if err != nil {
		return newError("csvEncodeRaw: %s", err.Error())
	}
	return stringToList(result)
}

func builtinCsvWrite(e *Evaluator, args ...Object) Object {
	if len(args) < 2 || len(args) > 3 {
		return newError("csvWrite(path, records, delimiter?)")
	}
	pathList, ok := args[0].(*List)
	if !ok {
		return newError("csvWrite: first argument must be String")
	}
	result, err := csvWrite(listToString(pathList), args[1], getDelimiter(args, 2))
	if err != nil {
		return makeFailStr(err.Error())
	}
	return result
}

func builtinCsvWriteRaw(e *Evaluator, args ...Object) Object {
	if len(args) < 2 || len(args) > 3 {
		return newError("csvWriteRaw(path, rows, delimiter?)")
	}
	pathList, ok := args[0].(*List)
	if !ok {
		return newError("csvWriteRaw: first argument must be String")
	}
	result, err := csvWriteRaw(listToString(pathList), args[1], getDelimiter(args, 2))
	if err != nil {
		return makeFailStr(err.Error())
	}
	return result
}
