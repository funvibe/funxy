package evaluator

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
	"unsafe" // Added unsafe for pointer hashing

	"github.com/funvibe/funxy/internal/typesystem"

	_ "modernc.org/sqlite" // SQLite driver
)

// SQL handle types
type SqlDB struct {
	db     *sql.DB
	driver string
}

func (s *SqlDB) Type() ObjectType             { return "SqlDB" }
func (s *SqlDB) Inspect() string              { return fmt.Sprintf("<SqlDB:%s>", s.driver) }
func (s *SqlDB) RuntimeType() typesystem.Type { return typesystem.TCon{Name: "SqlDB"} }
func (s *SqlDB) Hash() uint32 {
	return uint32(uintptr(unsafe.Pointer(s)))
}

type SqlTx struct {
	tx     *sql.Tx
	driver string
}

func (s *SqlTx) Type() ObjectType             { return "SqlTx" }
func (s *SqlTx) Inspect() string              { return fmt.Sprintf("<SqlTx:%s>", s.driver) }
func (s *SqlTx) RuntimeType() typesystem.Type { return typesystem.TCon{Name: "SqlTx"} }
func (s *SqlTx) Hash() uint32 {
	return uint32(uintptr(unsafe.Pointer(s)))
}

// SqlValue ADT constructors
var (
	sqlNullCtor   string = "SqlNull"
	sqlIntCtor    string = "SqlInt"
	sqlFloatCtor  string = "SqlFloat"
	sqlStringCtor string = "SqlString"
	sqlBoolCtor   string = "SqlBool"
	sqlBytesCtor  string = "SqlBytes"
	sqlTimeCtor   string = "SqlTime"
	sqlBigIntCtor string = "SqlBigInt"
)

// Global registry of open databases for cleanup
var (
	sqlDBRegistry         = make(map[int64]*SqlDB)
	sqlDBNextID     int64 = 1
	sqlDBRegistryMu sync.Mutex
)

func registerSqlDB(db *SqlDB) int64 {
	sqlDBRegistryMu.Lock()
	defer sqlDBRegistryMu.Unlock()
	id := sqlDBNextID
	sqlDBNextID++
	sqlDBRegistry[id] = db
	return id
}

// convertPlaceholders converts $1, $2 style to ? for SQLite
func convertPlaceholders(query string) string {
	re := regexp.MustCompile(`\$(\d+)`)
	return re.ReplaceAllString(query, "?")
}

// goValueToSqlValue converts Go value to SqlValue ADT
func goValueToSqlValue(val interface{}) Object {
	if val == nil {
		return &DataInstance{Name: sqlNullCtor, TypeName: "SqlValue", Fields: []Object{}}
	}

	switch v := val.(type) {
	case int64:
		return &DataInstance{Name: sqlIntCtor, TypeName: "SqlValue", Fields: []Object{&Integer{Value: v}}}
	case float64:
		return &DataInstance{Name: sqlFloatCtor, TypeName: "SqlValue", Fields: []Object{&Float{Value: v}}}
	case string:
		return &DataInstance{Name: sqlStringCtor, TypeName: "SqlValue", Fields: []Object{stringToList(v)}}
	case bool:
		return &DataInstance{Name: sqlBoolCtor, TypeName: "SqlValue", Fields: []Object{&Boolean{Value: v}}}
	case []byte:
		return &DataInstance{Name: sqlBytesCtor, TypeName: "SqlValue", Fields: []Object{bytesFromSlice(v)}}
	case time.Time:
		// Convert to our Date record
		_, offset := v.Zone()
		offsetMinutes := offset / 60
		dateRecord := NewRecord(map[string]Object{
			"year":   &Integer{Value: int64(v.Year())},
			"month":  &Integer{Value: int64(v.Month())},
			"day":    &Integer{Value: int64(v.Day())},
			"hour":   &Integer{Value: int64(v.Hour())},
			"minute": &Integer{Value: int64(v.Minute())},
			"second": &Integer{Value: int64(v.Second())},
			"offset": &Integer{Value: int64(offsetMinutes)},
		})
		return &DataInstance{Name: sqlTimeCtor, TypeName: "SqlValue", Fields: []Object{dateRecord}}
	default:
		// Try to handle as string
		return &DataInstance{Name: sqlStringCtor, TypeName: "SqlValue", Fields: []Object{stringToList(fmt.Sprintf("%v", v))}}
	}
}

// sqlObjectToGoValue converts Funxy object to Go value for SQL parameters
func sqlObjectToGoValue(obj Object) interface{} {
	switch o := obj.(type) {
	case *Integer:
		return o.Value
	case *Float:
		return o.Value
	case *Boolean:
		return o.Value
	case *List:
		// String (List<Char>)
		return sqlListToString(o)
	case *Bytes:
		return o.data
	case *BigInt:
		return o.Value.String()
	case *Rational:
		f, _ := o.Value.Float64()
		return f
	case *DataInstance:
		// Handle SqlValue variants
		switch o.Name {
		case sqlNullCtor:
			return nil
		case sqlIntCtor, sqlFloatCtor, sqlStringCtor, sqlBoolCtor, sqlBytesCtor, sqlBigIntCtor:
			if len(o.Fields) > 0 {
				return sqlObjectToGoValue(o.Fields[0])
			}
			return nil
		default:
			return nil
		}
	case *RecordInstance:
		// Could be a Date
		if year := o.Get("year"); year != nil {
			if yearInt, ok := year.(*Integer); ok {
				month := o.Get("month").(*Integer).Value
				day := o.Get("day").(*Integer).Value
				hour := int64(0)
				minute := int64(0)
				second := int64(0)
				if h := o.Get("hour"); h != nil {
					hour = h.(*Integer).Value
				}
				if m := o.Get("minute"); m != nil {
					minute = m.(*Integer).Value
				}
				if s := o.Get("second"); s != nil {
					second = s.(*Integer).Value
				}
				return time.Date(int(yearInt.Value), time.Month(month), int(day),
					int(hour), int(minute), int(second), 0, time.UTC)
			}
		}
		return nil
	case *Nil:
		return nil
	default:
		return nil
	}
}

// sqlListToString converts List<Char> to Go string
func sqlListToString(list *List) string {
	var sb strings.Builder
	for _, elem := range list.ToSlice() {
		if ch, ok := elem.(*Char); ok {
			sb.WriteRune(rune(ch.Value))
		}
	}
	return sb.String()
}

// paramsListToGoValues converts List of params to []interface{}
func paramsListToGoValues(list *List) []interface{} {
	var result []interface{}
	for _, elem := range list.ToSlice() {
		result = append(result, sqlObjectToGoValue(elem))
	}
	return result
}

// rowToMap converts sql.Rows current row to Map<String, SqlValue>
func rowToMap(rows *sql.Rows) (Object, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return nil, err
	}

	// Build Map
	m := newMap()
	for i, col := range columns {
		key := stringToList(col)
		val := goValueToSqlValue(values[i])
		m = m.put(key, val)
	}

	return m, nil
}

// builtinSqlOpen opens a database connection
func builtinSqlOpen(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("sqlOpen expects 2 arguments (driver, dsn), got %d", len(args))
	}

	driver := objectToString(args[0])
	dsn := objectToString(args[1])

	// Validate driver
	switch driver {
	case "sqlite", "sqlite3":
		driver = "sqlite"
	default:
		return makeFailStr(fmt.Sprintf("unsupported driver: %s (only 'sqlite' is supported)", driver))
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return makeFailStr(err.Error())
	}

	// Test connection
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return makeFailStr(err.Error())
	}

	sqlDB := &SqlDB{db: db, driver: driver}
	registerSqlDB(sqlDB)

	return makeOk(sqlDB)
}

// builtinSqlClose closes a database connection
func builtinSqlClose(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("sqlClose expects 1 argument, got %d", len(args))
	}

	sqlDB, ok := args[0].(*SqlDB)
	if !ok {
		return makeFailStr("sqlClose expects SqlDB")
	}

	if err := sqlDB.db.Close(); err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&Nil{})
}

// builtinSqlPing pings the database
func builtinSqlPing(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("sqlPing expects 1 argument, got %d", len(args))
	}

	sqlDB, ok := args[0].(*SqlDB)
	if !ok {
		return makeFailStr("sqlPing expects SqlDB")
	}

	if err := sqlDB.db.Ping(); err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&Nil{})
}

// builtinSqlQuery executes a SELECT query
func builtinSqlQuery(e *Evaluator, args ...Object) Object {
	if len(args) < 2 || len(args) > 3 {
		return newError("sqlQuery expects 2-3 arguments (db, query, [params]), got %d", len(args))
	}

	sqlDB, ok := args[0].(*SqlDB)
	if !ok {
		return makeFailStr("sqlQuery expects SqlDB as first argument")
	}

	query := objectToString(args[1])
	query = convertPlaceholders(query)

	var params []interface{}
	if len(args) == 3 {
		paramList, ok := args[2].(*List)
		if !ok {
			return makeFailStr("sqlQuery expects List as third argument")
		}
		params = paramsListToGoValues(paramList)
	}

	rows, err := sqlDB.db.Query(query, params...)
	if err != nil {
		return makeFailStr(err.Error())
	}
	defer func() { _ = rows.Close() }()

	var results []Object
	for rows.Next() {
		row, err := rowToMap(rows)
		if err != nil {
			return makeFailStr(err.Error())
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(newList(results))
}

// builtinSqlQueryRow executes a SELECT query expecting one row
func builtinSqlQueryRow(e *Evaluator, args ...Object) Object {
	if len(args) < 2 || len(args) > 3 {
		return newError("sqlQueryRow expects 2-3 arguments (db, query, [params]), got %d", len(args))
	}

	sqlDB, ok := args[0].(*SqlDB)
	if !ok {
		return makeFailStr("sqlQueryRow expects SqlDB as first argument")
	}

	query := objectToString(args[1])
	query = convertPlaceholders(query)

	var params []interface{}
	if len(args) == 3 {
		paramList, ok := args[2].(*List)
		if !ok {
			return makeFailStr("sqlQueryRow expects List as third argument")
		}
		params = paramsListToGoValues(paramList)
	}

	rows, err := sqlDB.db.Query(query, params...)
	if err != nil {
		return makeFailStr(err.Error())
	}
	defer func() { _ = rows.Close() }()

	if rows.Next() {
		row, err := rowToMap(rows)
		if err != nil {
			return makeFailStr(err.Error())
		}
		return makeOk(makeSome(row))
	}

	if err := rows.Err(); err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(makeNone())
}

// builtinSqlExec executes INSERT/UPDATE/DELETE
func builtinSqlExec(e *Evaluator, args ...Object) Object {
	if len(args) < 2 || len(args) > 3 {
		return newError("sqlExec expects 2-3 arguments (db, query, [params]), got %d", len(args))
	}

	sqlDB, ok := args[0].(*SqlDB)
	if !ok {
		return makeFailStr("sqlExec expects SqlDB as first argument")
	}

	query := objectToString(args[1])
	query = convertPlaceholders(query)

	var params []interface{}
	if len(args) == 3 {
		paramList, ok := args[2].(*List)
		if !ok {
			return makeFailStr("sqlExec expects List as third argument")
		}
		params = paramsListToGoValues(paramList)
	}

	result, err := sqlDB.db.Exec(query, params...)
	if err != nil {
		return makeFailStr(err.Error())
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&Integer{Value: affected})
}

// builtinSqlLastInsertId gets the last insert ID
func builtinSqlLastInsertId(e *Evaluator, args ...Object) Object {
	if len(args) < 2 || len(args) > 3 {
		return newError("sqlLastInsertId expects 2-3 arguments (db, query, [params]), got %d", len(args))
	}

	sqlDB, ok := args[0].(*SqlDB)
	if !ok {
		return makeFailStr("sqlLastInsertId expects SqlDB as first argument")
	}

	query := objectToString(args[1])
	query = convertPlaceholders(query)

	var params []interface{}
	if len(args) == 3 {
		paramList, ok := args[2].(*List)
		if !ok {
			return makeFailStr("sqlLastInsertId expects List as third argument")
		}
		params = paramsListToGoValues(paramList)
	}

	result, err := sqlDB.db.Exec(query, params...)
	if err != nil {
		return makeFailStr(err.Error())
	}

	lastID, err := result.LastInsertId()
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&Integer{Value: lastID})
}

// Transaction functions

// builtinSqlBegin starts a transaction
func builtinSqlBegin(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("sqlBegin expects 1 argument, got %d", len(args))
	}

	sqlDB, ok := args[0].(*SqlDB)
	if !ok {
		return makeFailStr("sqlBegin expects SqlDB")
	}

	tx, err := sqlDB.db.Begin()
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&SqlTx{tx: tx, driver: sqlDB.driver})
}

// builtinSqlCommit commits a transaction
func builtinSqlCommit(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("sqlCommit expects 1 argument, got %d", len(args))
	}

	sqlTx, ok := args[0].(*SqlTx)
	if !ok {
		return makeFailStr("sqlCommit expects SqlTx")
	}

	if err := sqlTx.tx.Commit(); err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&Nil{})
}

// builtinSqlRollback rolls back a transaction
func builtinSqlRollback(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("sqlRollback expects 1 argument, got %d", len(args))
	}

	sqlTx, ok := args[0].(*SqlTx)
	if !ok {
		return makeFailStr("sqlRollback expects SqlTx")
	}

	if err := sqlTx.tx.Rollback(); err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&Nil{})
}

// builtinSqlTxQuery executes a SELECT in a transaction
func builtinSqlTxQuery(e *Evaluator, args ...Object) Object {
	if len(args) < 2 || len(args) > 3 {
		return newError("sqlTxQuery expects 2-3 arguments (tx, query, [params]), got %d", len(args))
	}

	sqlTx, ok := args[0].(*SqlTx)
	if !ok {
		return makeFailStr("sqlTxQuery expects SqlTx as first argument")
	}

	query := objectToString(args[1])
	query = convertPlaceholders(query)

	var params []interface{}
	if len(args) == 3 {
		paramList, ok := args[2].(*List)
		if !ok {
			return makeFailStr("sqlTxQuery expects List as third argument")
		}
		params = paramsListToGoValues(paramList)
	}

	rows, err := sqlTx.tx.Query(query, params...)
	if err != nil {
		return makeFailStr(err.Error())
	}
	defer func() { _ = rows.Close() }()

	var results []Object
	for rows.Next() {
		row, err := rowToMap(rows)
		if err != nil {
			return makeFailStr(err.Error())
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(newList(results))
}

// builtinSqlTxExec executes INSERT/UPDATE/DELETE in a transaction
func builtinSqlTxExec(e *Evaluator, args ...Object) Object {
	if len(args) < 2 || len(args) > 3 {
		return newError("sqlTxExec expects 2-3 arguments (tx, query, [params]), got %d", len(args))
	}

	sqlTx, ok := args[0].(*SqlTx)
	if !ok {
		return makeFailStr("sqlTxExec expects SqlTx as first argument")
	}

	query := objectToString(args[1])
	query = convertPlaceholders(query)

	var params []interface{}
	if len(args) == 3 {
		paramList, ok := args[2].(*List)
		if !ok {
			return makeFailStr("sqlTxExec expects List as third argument")
		}
		params = paramsListToGoValues(paramList)
	}

	result, err := sqlTx.tx.Exec(query, params...)
	if err != nil {
		return makeFailStr(err.Error())
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&Integer{Value: affected})
}

// Helper: extract SqlValue from variant
func builtinSqlUnwrap(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("sqlUnwrap expects 1 argument, got %d", len(args))
	}

	dataInst, ok := args[0].(*DataInstance)
	if !ok {
		return args[0] // Not a DataInstance, return as-is
	}

	switch dataInst.Name {
	case sqlNullCtor:
		return makeNone() // Return Option None for null
	case sqlIntCtor, sqlFloatCtor, sqlStringCtor, sqlBoolCtor, sqlBytesCtor, sqlTimeCtor, sqlBigIntCtor:
		if len(dataInst.Fields) > 0 {
			return makeSome(dataInst.Fields[0])
		}
		return makeNone()
	default:
		return args[0]
	}
}

// Utility functions

// builtinSqlIsNull checks if a value is SqlNull
func builtinSqlIsNull(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("sqlIsNull expects 1 argument, got %d", len(args))
	}

	dataInst, ok := args[0].(*DataInstance)
	if !ok {
		return &Boolean{Value: false}
	}

	return &Boolean{Value: dataInst.Name == sqlNullCtor}
}

// SqlBuiltins returns all SQL builtins
func SqlBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		// Connection
		"sqlOpen":  {Fn: builtinSqlOpen},
		"sqlClose": {Fn: builtinSqlClose},
		"sqlPing":  {Fn: builtinSqlPing},

		// Query
		"sqlQuery":        {Fn: builtinSqlQuery},
		"sqlQueryRow":     {Fn: builtinSqlQueryRow},
		"sqlExec":         {Fn: builtinSqlExec},
		"sqlLastInsertId": {Fn: builtinSqlLastInsertId},

		// Transaction
		"sqlBegin":    {Fn: builtinSqlBegin},
		"sqlCommit":   {Fn: builtinSqlCommit},
		"sqlRollback": {Fn: builtinSqlRollback},
		"sqlTxQuery":  {Fn: builtinSqlTxQuery},
		"sqlTxExec":   {Fn: builtinSqlTxExec},

		// Utility
		"sqlUnwrap": {Fn: builtinSqlUnwrap},
		"sqlIsNull": {Fn: builtinSqlIsNull},
	}
}

// RegisterSqlBuiltins registers SQL types and functions into an environment
func RegisterSqlBuiltins(env *Environment) {
	// Types
	env.Set("SqlValue", &TypeObject{TypeVal: typesystem.TCon{Name: "SqlValue"}})
	env.Set("SqlDB", &TypeObject{TypeVal: typesystem.TCon{Name: "SqlDB"}})
	env.Set("SqlTx", &TypeObject{TypeVal: typesystem.TCon{Name: "SqlTx"}})

	// Constructors
	env.Set("SqlNull", &DataInstance{Name: "SqlNull", Fields: []Object{}, TypeName: "SqlValue"})
	env.Set("SqlInt", &Constructor{Name: "SqlInt", TypeName: "SqlValue", Arity: 1})
	env.Set("SqlFloat", &Constructor{Name: "SqlFloat", TypeName: "SqlValue", Arity: 1})
	env.Set("SqlString", &Constructor{Name: "SqlString", TypeName: "SqlValue", Arity: 1})
	env.Set("SqlBool", &Constructor{Name: "SqlBool", TypeName: "SqlValue", Arity: 1})
	env.Set("SqlBytes", &Constructor{Name: "SqlBytes", TypeName: "SqlValue", Arity: 1})
	env.Set("SqlTime", &Constructor{Name: "SqlTime", TypeName: "SqlValue", Arity: 1})
	env.Set("SqlBigInt", &Constructor{Name: "SqlBigInt", TypeName: "SqlValue", Arity: 1})

	// Functions
	builtins := SqlBuiltins()
	SetSqlBuiltinTypes(builtins)
	for name, fn := range builtins {
		env.Set(name, fn)
	}
}

// SetSqlBuiltinTypes sets TypeInfo for SQL builtins
func SetSqlBuiltinTypes(builtins map[string]*Builtin) {
	stringType := typesystem.TApp{Constructor: typesystem.TCon{Name: "List"}, Args: []typesystem.Type{typesystem.Char}}
	nilType := typesystem.Nil
	intType := typesystem.Int
	boolType := typesystem.Bool

	sqlDBType := typesystem.TCon{Name: "SqlDB"}
	sqlTxType := typesystem.TCon{Name: "SqlTx"}
	sqlValueType := typesystem.TCon{Name: "SqlValue"}

	resultDB := typesystem.TApp{Constructor: typesystem.TCon{Name: "Result"}, Args: []typesystem.Type{stringType, sqlDBType}}
	resultTx := typesystem.TApp{Constructor: typesystem.TCon{Name: "Result"}, Args: []typesystem.Type{stringType, sqlTxType}}
	resultNil := typesystem.TApp{Constructor: typesystem.TCon{Name: "Result"}, Args: []typesystem.Type{stringType, nilType}}
	resultInt := typesystem.TApp{Constructor: typesystem.TCon{Name: "Result"}, Args: []typesystem.Type{stringType, intType}}

	rowType := typesystem.TApp{Constructor: typesystem.TCon{Name: "Map"}, Args: []typesystem.Type{stringType, sqlValueType}}
	listRow := typesystem.TApp{Constructor: typesystem.TCon{Name: "List"}, Args: []typesystem.Type{rowType}}
	optionRow := typesystem.TApp{Constructor: typesystem.TCon{Name: "Option"}, Args: []typesystem.Type{rowType}}
	resultListRow := typesystem.TApp{Constructor: typesystem.TCon{Name: "Result"}, Args: []typesystem.Type{stringType, listRow}}
	resultOptionRow := typesystem.TApp{Constructor: typesystem.TCon{Name: "Result"}, Args: []typesystem.Type{stringType, optionRow}}
	optionSqlValue := typesystem.TApp{Constructor: typesystem.TCon{Name: "Option"}, Args: []typesystem.Type{sqlValueType}}

	anyType := typesystem.TVar{Name: "a"}
	paramsType := typesystem.TApp{Constructor: typesystem.TCon{Name: "List"}, Args: []typesystem.Type{anyType}}

	types := map[string]typesystem.Type{
		"sqlOpen":         typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: resultDB},
		"sqlClose":        typesystem.TFunc{Params: []typesystem.Type{sqlDBType}, ReturnType: resultNil},
		"sqlPing":         typesystem.TFunc{Params: []typesystem.Type{sqlDBType}, ReturnType: resultNil},
		"sqlQuery":        typesystem.TFunc{Params: []typesystem.Type{sqlDBType, stringType, paramsType}, ReturnType: resultListRow},
		"sqlQueryRow":     typesystem.TFunc{Params: []typesystem.Type{sqlDBType, stringType, paramsType}, ReturnType: resultOptionRow},
		"sqlExec":         typesystem.TFunc{Params: []typesystem.Type{sqlDBType, stringType, paramsType}, ReturnType: resultInt},
		"sqlLastInsertId": typesystem.TFunc{Params: []typesystem.Type{sqlDBType, stringType, paramsType}, ReturnType: resultInt},
		"sqlBegin":        typesystem.TFunc{Params: []typesystem.Type{sqlDBType}, ReturnType: resultTx},
		"sqlCommit":       typesystem.TFunc{Params: []typesystem.Type{sqlTxType}, ReturnType: resultNil},
		"sqlRollback":     typesystem.TFunc{Params: []typesystem.Type{sqlTxType}, ReturnType: resultNil},
		"sqlTxQuery":      typesystem.TFunc{Params: []typesystem.Type{sqlTxType, stringType, paramsType}, ReturnType: resultListRow},
		"sqlTxExec":       typesystem.TFunc{Params: []typesystem.Type{sqlTxType, stringType, paramsType}, ReturnType: resultInt},
		"sqlUnwrap":       typesystem.TFunc{Params: []typesystem.Type{sqlValueType}, ReturnType: optionSqlValue},
		"sqlIsNull":       typesystem.TFunc{Params: []typesystem.Type{sqlValueType}, ReturnType: boolType},
	}

	for name, builtin := range builtins {
		if t, ok := types[name]; ok {
			builtin.TypeInfo = t
		}
	}
}
