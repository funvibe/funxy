package evaluator

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/funvibe/funxy/internal/typesystem"
)

// Log levels
const (
	LogLevelDebug = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelFatal
)

var logLevelNames = map[int]string{
	LogLevelDebug: "DEBUG",
	LogLevelInfo:  "INFO",
	LogLevelWarn:  "WARN",
	LogLevelError: "ERROR",
	LogLevelFatal: "FATAL",
}

var logLevelFromString = map[string]int{
	"debug": LogLevelDebug,
	"info":  LogLevelInfo,
	"warn":  LogLevelWarn,
	"error": LogLevelError,
	"fatal": LogLevelFatal,
}

// Logger configuration
type LoggerConfig struct {
	mu       sync.Mutex
	level    int
	format   string // "text" or "json"
	output   io.Writer
	prefix   string
	useColor bool
	isFile   bool // true if output is a file (not stderr/stdout)
}

// Global logger
var globalLogger = &LoggerConfig{
	level:    LogLevelInfo,
	format:   "text",
	output:   os.Stderr,
	prefix:   "",
	useColor: true,
}

// Logger object for prefixed loggers
type Logger struct {
	prefix string
}

func (l *Logger) Type() ObjectType             { return "LOGGER" }
func (l *Logger) TypeName() string             { return "Logger" }
func (l *Logger) Inspect() string              { return fmt.Sprintf("Logger{prefix=%q}", l.prefix) }
func (l *Logger) RuntimeType() typesystem.Type { return typesystem.TCon{Name: "Logger"} }
func (l *Logger) Hash() uint32 {
	return hashString(l.prefix)
}

// ANSI colors
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
)

func levelColor(level int) string {
	switch level {
	case LogLevelDebug:
		return colorGray
	case LogLevelInfo:
		return colorBlue
	case LogLevelWarn:
		return colorYellow
	case LogLevelError, LogLevelFatal:
		return colorRed
	default:
		return colorReset
	}
}

// LogBuiltins returns built-in functions for lib/log
func LogBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		// Basic logging
		"logDebug": {Fn: builtinLogDebug, Name: "logDebug"},
		"logInfo":  {Fn: builtinLogInfo, Name: "logInfo"},
		"logWarn":  {Fn: builtinLogWarn, Name: "logWarn"},
		"logError": {Fn: builtinLogError, Name: "logError"},
		"logFatal": {Fn: builtinLogFatal, Name: "logFatal"},

		// Fatal with exit
		"logFatalExit": {Fn: builtinLogFatalExit, Name: "logFatalExit"},

		// Configuration
		"logLevel":  {Fn: builtinLogLevel, Name: "logLevel"},
		"logFormat": {Fn: builtinLogFormat, Name: "logFormat"},
		"logOutput": {Fn: builtinLogOutput, Name: "logOutput"},
		"logColor":  {Fn: builtinLogColor, Name: "logColor"},

		// Structured logging
		"logWithFields": {Fn: builtinLogWithFields, Name: "logWithFields"},

		// Prefixed logger
		"logWithPrefix": {Fn: builtinLogWithPrefix, Name: "logWithPrefix"},

		// Logger methods (for prefixed loggers)
		"loggerDebug":      {Fn: builtinLoggerDebug, Name: "loggerDebug"},
		"loggerInfo":       {Fn: builtinLoggerInfo, Name: "loggerInfo"},
		"loggerWarn":       {Fn: builtinLoggerWarn, Name: "loggerWarn"},
		"loggerError":      {Fn: builtinLoggerError, Name: "loggerError"},
		"loggerFatal":      {Fn: builtinLoggerFatal, Name: "loggerFatal"},
		"loggerFatalExit":  {Fn: builtinLoggerFatalExit, Name: "loggerFatalExit"},
		"loggerWithFields": {Fn: builtinLoggerWithFields, Name: "loggerWithFields"},
	}
}

// SetLogBuiltinTypes sets type information for log builtins
func SetLogBuiltinTypes(builtins map[string]*Builtin) {
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}
	loggerType := typesystem.TCon{Name: "Logger"}
	nilType := typesystem.Nil
	boolType := typesystem.Bool

	// Map<String, a> for fields
	mapStringAny := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Map"},
		Args:        []typesystem.Type{stringType, typesystem.TVar{Name: "a"}},
	}

	resultNil := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, nilType},
	}

	types := map[string]typesystem.Type{
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
	}

	for name, typ := range types {
		if b, ok := builtins[name]; ok {
			b.TypeInfo = typ
		}
	}
}

// doLogWithPrefix logs with the global config but a custom prefix
func doLogWithPrefix(prefix string, level int, msg string, fields map[string]interface{}) {
	globalLogger.mu.Lock()
	defer globalLogger.mu.Unlock()

	if level < globalLogger.level {
		return
	}

	doLogInternal(globalLogger, prefix, level, msg, fields)
}

// Core logging function
func doLog(config *LoggerConfig, level int, msg string, fields map[string]interface{}) {
	config.mu.Lock()
	defer config.mu.Unlock()

	if level < config.level {
		return
	}

	doLogInternal(config, config.prefix, level, msg, fields)
}

// Internal log function
func doLogInternal(config *LoggerConfig, prefix string, level int, msg string, fields map[string]interface{}) {

	timestamp := time.Now().UTC().Format(time.RFC3339)

	if config.format == "json" {
		entry := map[string]interface{}{
			"time":  timestamp,
			"level": strings.ToLower(logLevelNames[level]),
			"msg":   msg,
		}
		if prefix != "" {
			entry["prefix"] = prefix
		}
		for k, v := range fields {
			entry[k] = v
		}
		data, _ := json.Marshal(entry)
		_, _ = fmt.Fprintln(config.output, string(data))
	} else {
		// Text format
		var sb strings.Builder

		// Color for level
		if config.useColor {
			sb.WriteString(levelColor(level))
		}

		sb.WriteString(timestamp)
		sb.WriteString(" ")
		sb.WriteString(logLevelNames[level])

		if config.useColor {
			sb.WriteString(colorReset)
		}

		if prefix != "" {
			sb.WriteString(" [")
			sb.WriteString(prefix)
			sb.WriteString("]")
		}

		sb.WriteString(" ")
		sb.WriteString(msg)

		// Fields
		if len(fields) > 0 {
			// Sort keys for consistent output
			keys := make([]string, 0, len(fields))
			for k := range fields {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for _, k := range keys {
				sb.WriteString(" ")
				sb.WriteString(k)
				sb.WriteString("=")
				sb.WriteString(fmt.Sprintf("%v", fields[k]))
			}
		}

		_, _ = fmt.Fprintln(config.output, sb.String())
	}
}

// ===== Basic logging =====

func builtinLogDebug(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("logDebug expects 1 argument, got %d", len(args))
	}
	msg := objectToString(args[0])
	doLog(globalLogger, LogLevelDebug, msg, nil)
	return &Nil{}
}

func builtinLogInfo(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("logInfo expects 1 argument, got %d", len(args))
	}
	msg := objectToString(args[0])
	doLog(globalLogger, LogLevelInfo, msg, nil)
	return &Nil{}
}

func builtinLogWarn(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("logWarn expects 1 argument, got %d", len(args))
	}
	msg := objectToString(args[0])
	doLog(globalLogger, LogLevelWarn, msg, nil)
	return &Nil{}
}

func builtinLogError(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("logError expects 1 argument, got %d", len(args))
	}
	msg := objectToString(args[0])
	doLog(globalLogger, LogLevelError, msg, nil)
	return &Nil{}
}

func builtinLogFatal(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("logFatal expects 1 argument, got %d", len(args))
	}
	msg := objectToString(args[0])
	doLog(globalLogger, LogLevelFatal, msg, nil)
	return &Nil{}
}

func builtinLogFatalExit(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("logFatalExit expects 1 argument, got %d", len(args))
	}
	msg := objectToString(args[0])
	doLog(globalLogger, LogLevelFatal, msg, nil)
	os.Exit(1)
	return &Nil{}
}

// ===== Configuration =====

func builtinLogLevel(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("logLevel expects 1 argument, got %d", len(args))
	}
	levelStr := strings.ToLower(objectToString(args[0]))
	level, ok := logLevelFromString[levelStr]
	if !ok {
		return newError("logLevel: invalid level %q, expected debug|info|warn|error|fatal", levelStr)
	}
	globalLogger.mu.Lock()
	globalLogger.level = level
	globalLogger.mu.Unlock()
	return &Nil{}
}

func builtinLogFormat(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("logFormat expects 1 argument, got %d", len(args))
	}
	format := strings.ToLower(objectToString(args[0]))
	if format != "text" && format != "json" {
		return newError("logFormat: invalid format %q, expected text|json", format)
	}
	globalLogger.mu.Lock()
	globalLogger.format = format
	globalLogger.mu.Unlock()
	return &Nil{}
}

func builtinLogOutput(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("logOutput expects 1 argument, got %d", len(args))
	}
	path := objectToString(args[0])

	// Special cases
	if path == "stderr" {
		globalLogger.mu.Lock()
		globalLogger.output = os.Stderr
		globalLogger.isFile = false
		globalLogger.mu.Unlock()
		return makeOk(&Nil{})
	}
	if path == "stdout" {
		globalLogger.mu.Lock()
		globalLogger.output = os.Stdout
		globalLogger.isFile = false
		globalLogger.mu.Unlock()
		return makeOk(&Nil{})
	}

	// Open file
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return makeFailStr(err.Error())
	}

	globalLogger.mu.Lock()
	globalLogger.output = f
	globalLogger.useColor = false // Disable colors for file output
	globalLogger.isFile = true
	globalLogger.mu.Unlock()

	return makeOk(&Nil{})
}

func builtinLogColor(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("logColor expects 1 argument, got %d", len(args))
	}
	enabled, ok := args[0].(*Boolean)
	if !ok {
		return newError("logColor: argument must be Bool, got %s", args[0].Type())
	}
	globalLogger.mu.Lock()
	// Ignore color enable when output is a file
	if enabled.Value && globalLogger.isFile {
		globalLogger.mu.Unlock()
		return &Nil{}
	}
	globalLogger.useColor = enabled.Value
	globalLogger.mu.Unlock()
	return &Nil{}
}

// ===== Structured logging =====

func builtinLogWithFields(e *Evaluator, args ...Object) Object {
	if len(args) != 3 {
		return newError("logWithFields expects 3 arguments (level, msg, fields), got %d", len(args))
	}

	levelStr := strings.ToLower(objectToString(args[0]))
	level, ok := logLevelFromString[levelStr]
	if !ok {
		return newError("logWithFields: invalid level %q", levelStr)
	}

	msg := objectToString(args[1])

	fields := extractFields(args[2])

	doLog(globalLogger, level, msg, fields)
	return &Nil{}
}

func extractFields(obj Object) map[string]interface{} {
	fields := make(map[string]interface{})

	m, ok := obj.(*Map)
	if !ok {
		return fields
	}

	// m.items() returns *List of tuples
	itemsList := m.items()
	for i := 0; i < itemsList.len(); i++ {
		tuple, ok := itemsList.get(i).(*Tuple)
		if !ok || len(tuple.Elements) != 2 {
			continue
		}
		key := objectToString(tuple.Elements[0])
		fields[key] = logObjectToGoValue(tuple.Elements[1])
	}

	return fields
}

func logObjectToGoValue(obj Object) interface{} {
	switch v := obj.(type) {
	case *Integer:
		return v.Value
	case *Float:
		return v.Value
	case *Boolean:
		return v.Value
	case *List:
		return listToString(v)
	case *Nil:
		return nil
	default:
		return obj.Inspect()
	}
}

// ===== Prefixed logger =====

func builtinLogWithPrefix(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("logWithPrefix expects 1 argument, got %d", len(args))
	}

	prefix := objectToString(args[0])

	// Prefixed logger only stores prefix, uses global settings for everything else
	return &Logger{prefix: prefix}
}

// ===== Logger methods =====

func builtinLoggerDebug(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("loggerDebug expects 2 arguments, got %d", len(args))
	}
	logger, ok := args[0].(*Logger)
	if !ok {
		return newError("loggerDebug: first argument must be Logger, got %s", args[0].Type())
	}
	msg := objectToString(args[1])
	doLogWithPrefix(logger.prefix, LogLevelDebug, msg, nil)
	return &Nil{}
}

func builtinLoggerInfo(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("loggerInfo expects 2 arguments, got %d", len(args))
	}
	logger, ok := args[0].(*Logger)
	if !ok {
		return newError("loggerInfo: first argument must be Logger, got %s", args[0].Type())
	}
	msg := objectToString(args[1])
	doLogWithPrefix(logger.prefix, LogLevelInfo, msg, nil)
	return &Nil{}
}

func builtinLoggerWarn(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("loggerWarn expects 2 arguments, got %d", len(args))
	}
	logger, ok := args[0].(*Logger)
	if !ok {
		return newError("loggerWarn: first argument must be Logger, got %s", args[0].Type())
	}
	msg := objectToString(args[1])
	doLogWithPrefix(logger.prefix, LogLevelWarn, msg, nil)
	return &Nil{}
}

func builtinLoggerError(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("loggerError expects 2 arguments, got %d", len(args))
	}
	logger, ok := args[0].(*Logger)
	if !ok {
		return newError("loggerError: first argument must be Logger, got %s", args[0].Type())
	}
	msg := objectToString(args[1])
	doLogWithPrefix(logger.prefix, LogLevelError, msg, nil)
	return &Nil{}
}

func builtinLoggerFatal(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("loggerFatal expects 2 arguments, got %d", len(args))
	}
	logger, ok := args[0].(*Logger)
	if !ok {
		return newError("loggerFatal: first argument must be Logger, got %s", args[0].Type())
	}
	msg := objectToString(args[1])
	doLogWithPrefix(logger.prefix, LogLevelFatal, msg, nil)
	return &Nil{}
}

func builtinLoggerFatalExit(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("loggerFatalExit expects 2 arguments, got %d", len(args))
	}
	logger, ok := args[0].(*Logger)
	if !ok {
		return newError("loggerFatalExit: first argument must be Logger, got %s", args[0].Type())
	}
	msg := objectToString(args[1])
	doLogWithPrefix(logger.prefix, LogLevelFatal, msg, nil)
	os.Exit(1)
	return &Nil{}
}

func builtinLoggerWithFields(e *Evaluator, args ...Object) Object {
	if len(args) != 4 {
		return newError("loggerWithFields expects 4 arguments (logger, level, msg, fields), got %d", len(args))
	}

	logger, ok := args[0].(*Logger)
	if !ok {
		return newError("loggerWithFields: first argument must be Logger, got %s", args[0].Type())
	}

	levelStr := strings.ToLower(objectToString(args[1]))
	level, ok := logLevelFromString[levelStr]
	if !ok {
		return newError("loggerWithFields: invalid level %q", levelStr)
	}

	msg := objectToString(args[2])
	fields := extractFields(args[3])

	doLogWithPrefix(logger.prefix, level, msg, fields)
	return &Nil{}
}

