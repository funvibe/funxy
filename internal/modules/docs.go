package modules

import (
	"fmt"
	"github.com/funvibe/funxy/internal/config"
	"sort"
	"strings"
)

// DocEntry represents documentation for a single item (function, type, operator)
type DocEntry struct {
	Name        string // e.g., "map", "Int", "+"
	Signature   string // e.g., "((A) -> B, List<A>) -> List<B>"
	Description string // e.g., "Apply function to each element"
	Example     string // e.g., "map(fn, [1,2,3])"
	Category    string // e.g., "Higher-Order", "Access", "Arithmetic"
}

// DocPackage represents documentation for a package
type DocPackage struct {
	Path        string      // e.g., "lib/list", "prelude"
	Description string      // Package description
	Functions   []*DocEntry // Functions in the package
	Types       []*DocEntry // Types defined in package
	Traits      []*DocEntry // Traits defined in package
	Operators   []*DocEntry // Operators defined in package
}

// docPackages stores all documentation
var docPackages = map[string]*DocPackage{}

// RegisterDocPackage registers package documentation
func RegisterDocPackage(pkg *DocPackage) {
	docPackages[pkg.Path] = pkg
}

// GetDocPackage returns documentation for a package
func GetDocPackage(path string) *DocPackage {
	return docPackages[path]
}

// GetAllDocPackages returns all documented packages
func GetAllDocPackages() []*DocPackage {
	var result []*DocPackage
	for _, pkg := range docPackages {
		result = append(result, pkg)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Path < result[j].Path
	})
	return result
}

// SearchDocs searches for a term in all documentation
func SearchDocs(term string) []*DocEntry {
	term = strings.ToLower(term)
	var results []*DocEntry

	for _, pkg := range docPackages {
		for _, entry := range pkg.Functions {
			if matchesSearch(entry, term) {
				results = append(results, entry)
			}
		}
		for _, entry := range pkg.Types {
			if matchesSearch(entry, term) {
				results = append(results, entry)
			}
		}
		for _, entry := range pkg.Traits {
			if matchesSearch(entry, term) {
				results = append(results, entry)
			}
		}
		for _, entry := range pkg.Operators {
			if matchesSearch(entry, term) {
				results = append(results, entry)
			}
		}
	}

	// Sort: exact matches first, then by name
	sort.Slice(results, func(i, j int) bool {
		nameI := strings.ToLower(results[i].Name)
		nameJ := strings.ToLower(results[j].Name)
		exactI := nameI == term
		exactJ := nameJ == term
		if exactI != exactJ {
			return exactI // exact match comes first
		}
		return nameI < nameJ // alphabetical
	})

	return results
}

func matchesSearch(entry *DocEntry, term string) bool {
	return strings.Contains(strings.ToLower(entry.Name), term) ||
		strings.Contains(strings.ToLower(entry.Description), term)
}

// FormatDocEntry formats a single doc entry for display
func FormatDocEntry(e *DocEntry) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("  %s", e.Name))
	if e.Signature != "" {
		sb.WriteString(fmt.Sprintf(" : %s", e.Signature))
	}
	sb.WriteString("\n")
	if e.Description != "" {
		sb.WriteString(fmt.Sprintf("    %s\n", e.Description))
	}
	if e.Example != "" {
		sb.WriteString(fmt.Sprintf("    Example: %s\n", e.Example))
	}
	return sb.String()
}

// FormatDocPackage formats package documentation for display
func FormatDocPackage(pkg *DocPackage) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("=== %s ===\n", pkg.Path))
	if pkg.Description != "" {
		sb.WriteString(fmt.Sprintf("%s\n", pkg.Description))
	}
	sb.WriteString("\n")

	if len(pkg.Types) > 0 {
		sb.WriteString("Types:\n")
		for _, t := range pkg.Types {
			sb.WriteString(FormatDocEntry(t))
		}
		sb.WriteString("\n")
	}

	if len(pkg.Traits) > 0 {
		sb.WriteString("Traits:\n")
		for _, t := range pkg.Traits {
			sb.WriteString(FormatDocEntry(t))
		}
		sb.WriteString("\n")
	}

	if len(pkg.Operators) > 0 {
		sb.WriteString("Operators:\n")
		for _, op := range pkg.Operators {
			sb.WriteString(FormatDocEntry(op))
		}
		sb.WriteString("\n")
	}

	if len(pkg.Functions) > 0 {
		sb.WriteString("Functions:\n")
		// Group by category
		categories := make(map[string][]*DocEntry)
		for _, f := range pkg.Functions {
			cat := f.Category
			if cat == "" {
				cat = "Other"
			}
			categories[cat] = append(categories[cat], f)
		}

		// Sort categories
		var catNames []string
		for name := range categories {
			catNames = append(catNames, name)
		}
		sort.Strings(catNames)

		for _, cat := range catNames {
			sb.WriteString(fmt.Sprintf("\n  [%s]\n", cat))
			for _, f := range categories[cat] {
				sb.WriteString(FormatDocEntry(f))
			}
		}
	}

	return sb.String()
}

// PrintHelp prints general help
func PrintHelp() string {
	var sb strings.Builder
	sb.WriteString("Funxy - A Hybrid Programming Language\n")
	sb.WriteString("=========================================\n\n")
	sb.WriteString("Usage:\n")
	sb.WriteString("  funxy <file>                Run a program\n")
	sb.WriteString("  funxy -c <file>             Compile to bytecode (.fbc)\n")
	sb.WriteString("  funxy -r <file>             Run compiled bytecode (.fbc)\n")
	sb.WriteString("  funxy -help                 Show this help\n")
	sb.WriteString("  funxy -help packages        Show lib packages\n")
	sb.WriteString("  funxy -help <package>       Show package documentation\n")
	sb.WriteString("  funxy -help search <term>   Search documentation\n")
	sb.WriteString("  funxy -help precedence      Show operator precedence\n")
	sb.WriteString("\n")
	sb.WriteString("File extensions: .lang, .funxy, .fx\n")
	sb.WriteString("\n")
	sb.WriteString("Note: Bytecode compilation (-c) works for single-file programs.\n")
	sb.WriteString("      Module imports are not yet supported in compiled bytecode.\n")
	return sb.String()
}

// PrintPrecedence prints operator precedence table (generated from config.AllOperators)
func PrintPrecedence() string {
	var sb strings.Builder
	sb.WriteString("Operator Precedence (higher = binds tighter)\n")
	sb.WriteString("=============================================\n\n")
	sb.WriteString("Prec  Assoc   Operators\n")
	sb.WriteString("----  -----   ---------\n")

	// Group by precedence
	byPrec := config.GetOperatorsByPrecedence()

	// Print from highest to lowest
	for prec := len(byPrec) - 1; prec >= 0; prec-- {
		ops := byPrec[prec]
		if len(ops) == 0 {
			continue
		}

		// Group by category within precedence
		byCategory := make(map[string][]string)
		assoc := "left"
		for _, op := range ops {
			byCategory[op.Category] = append(byCategory[op.Category], op.Symbol)
			if op.Assoc == config.AssocRight {
				assoc = "right"
			}
		}

		// Build operator string with categories
		var parts []string
		for cat, syms := range byCategory {
			if cat == "User" {
				// Skip user operators in main display, show separately
				continue
			}
			parts = append(parts, fmt.Sprintf("%s (%s)", strings.Join(syms, " "), cat))
		}

		if len(parts) > 0 {
			sb.WriteString(fmt.Sprintf(" %2d   %-7s %s\n", prec, assoc, strings.Join(parts, ", ")))
		}
	}

	sb.WriteString("\nUser-definable operators (implement trait to use):\n")
	var userOps []string
	for _, op := range config.AllOperators {
		if op.Category == "User" {
			userOps = append(userOps, fmt.Sprintf("  %s -> %s", op.Symbol, op.Trait))
		}
	}
	sb.WriteString(strings.Join(userOps, "\n"))
	sb.WriteString("\n")

	sb.WriteString("\nOverriding operators:\n")
	sb.WriteString("  Trait-bound operators can be overridden by implementing the trait:\n")
	sb.WriteString("    instance Numeric Vec2 { operator (+)(a, b) -> ... }\n")
	sb.WriteString("\n")
	sb.WriteString("  Built-in operators cannot be overridden: ")

	var builtins []string
	for _, op := range config.AllOperators {
		if !op.CanOverride {
			builtins = append(builtins, op.Symbol)
		}
	}
	sb.WriteString(strings.Join(builtins, " "))
	sb.WriteString("\n")

	return sb.String()
}

// InitDocumentation initializes all documentation
func InitDocumentation() {
	initPreludeDocs()
	initListDocs()
	initMapDocs()
	initBytesDocs()
	initBitsDocs()
	initStringDocs()
	initTimeDocs()
	initIODocs()
	initSysDocs()
	initTupleDocs()
	initMathDocs()
	initBignumDocs()
	initCharDocs()
	initJsonDocs()
	initCryptoDocs()
	initRegexDocs()
	initHttpDocs()
	initTestDocs()
	initRandDocs()
	initDateDocs()
	initWsDocs()
	initSqlDocs()
	initUrlDocs()
	initPathDocs()
	initUuidDocs()
	initLogDocs()
	initTaskDocs()
	initCsvDocs()
	initFlagDocs()

	// Auto-generate docs for any packages that were registered but not documented
	autoGenerateMissingDocs()
}

// autoGenerateMissingDocs creates basic documentation for registered packages
// that don't have manual documentation. This ensures new packages are always visible.
func autoGenerateMissingDocs() {
	for path, vp := range virtualPackages {
		// Skip if already documented
		if _, exists := docPackages[path]; exists {
			continue
		}

		// Skip meta-packages
		if path == "lib" {
			continue
		}

		// Generate basic docs from VirtualPackage symbols
		pkg := &DocPackage{
			Path:        path,
			Description: vp.Name + " package (auto-generated docs)",
			Functions:   []*DocEntry{},
		}

		for name, typ := range vp.Symbols {
			pkg.Functions = append(pkg.Functions, &DocEntry{
				Name:        name,
				Signature:   typ.String(),
				Description: "",
			})
		}

		RegisterDocPackage(pkg)
	}
}

// DocMeta contains only the descriptive metadata for a function (no signature - that comes from VirtualPackage)
type DocMeta struct {
	Description string
	Example     string
	Category    string
}

// generatePackageDocs creates documentation by merging VirtualPackage symbols with manual metadata.
// This is the single source of truth: signatures come from VirtualPackage, descriptions from meta.
func generatePackageDocs(path string, description string, meta map[string]*DocMeta, types []*DocEntry) *DocPackage {
	vp := GetVirtualPackage(path)
	if vp == nil {
		return &DocPackage{Path: path, Description: description}
	}

	pkg := &DocPackage{
		Path:        path,
		Description: description,
		Functions:   []*DocEntry{},
		Types:       types,
	}

	// Generate function docs from VirtualPackage.Symbols
	for name, typ := range vp.Symbols {
		entry := &DocEntry{
			Name:      name,
			Signature: typ.String(),
		}

		// Merge with manual metadata if available
		if m, ok := meta[name]; ok {
			entry.Description = m.Description
			entry.Example = m.Example
			entry.Category = m.Category
		}

		pkg.Functions = append(pkg.Functions, entry)
	}

	// Sort functions by category then name for consistent output
	sort.Slice(pkg.Functions, func(i, j int) bool {
		if pkg.Functions[i].Category != pkg.Functions[j].Category {
			return pkg.Functions[i].Category < pkg.Functions[j].Category
		}
		return pkg.Functions[i].Name < pkg.Functions[j].Name
	})

	return pkg
}

// generateTypeDocs generates type documentation from config.BuiltinTypes
func generateTypeDocs() []*DocEntry {
	var entries []*DocEntry
	for _, t := range config.BuiltinTypes {
		name := t.Name
		if t.Kind == "* -> *" {
			name = t.Name + "<T>"
		} else if t.Kind == "* -> * -> *" {
			name = t.Name + "<T, E>"
		}
		entries = append(entries, &DocEntry{
			Name:        name,
			Signature:   t.Kind,
			Description: t.Description,
			Example:     t.Example,
		})
	}
	return entries
}

// generateTraitDocs generates trait documentation from config.BuiltinTraits
func generateTraitDocs() []*DocEntry {
	var entries []*DocEntry
	for _, t := range config.BuiltinTraits {
		// Build signature from operators/methods
		var sig []string
		sig = append(sig, t.Operators...)
		sig = append(sig, t.Methods...)
		sigStr := strings.Join(sig, ", ")

		// Build description with inheritance
		desc := t.Description
		if len(t.SuperTraits) > 0 {
			desc = fmt.Sprintf("%s (extends %s)", desc, strings.Join(t.SuperTraits, ", "))
		}
		if t.Kind == "* -> *" {
			desc = fmt.Sprintf("%s [HKT]", desc)
		}

		name := fmt.Sprintf("%s<%s>", t.Name, strings.Join(t.TypeParams, ", "))

		entries = append(entries, &DocEntry{
			Name:        name,
			Signature:   sigStr,
			Description: desc,
		})
	}
	return entries
}

// generateOperatorDocs generates operator documentation from config.AllOperators
func generateOperatorDocs() []*DocEntry {
	var entries []*DocEntry

	for _, op := range config.AllOperators {
		// Build signature with constraint
		sig := op.Signature
		if op.Trait != "" && op.CanOverride {
			sig = fmt.Sprintf("%s where %s", op.Signature, op.Trait)
		}

		// Build description
		desc := op.Description
		if op.CanOverride && op.Trait != "" {
			desc = fmt.Sprintf("%s. Override: instance %s MyType", op.Description, op.Trait)
		} else if !op.CanOverride {
			desc = fmt.Sprintf("%s. Built-in, cannot override", op.Description)
		}

		// Build category with precedence
		assoc := "left"
		if op.Assoc == config.AssocRight {
			assoc = "right"
		}
		category := fmt.Sprintf("%s (prec %d, %s)", op.Category, op.Precedence, assoc)

		entries = append(entries, &DocEntry{
			Name:        op.Symbol,
			Signature:   sig,
			Description: desc,
			Category:    category,
		})
	}

	return entries
}

// generateFunctionDocs generates function documentation from config.BuiltinFunctions
func generateFunctionDocs() []*DocEntry {
	var entries []*DocEntry
	for _, fn := range config.BuiltinFunctions {
		sig := fn.Signature
		if fn.Constraint != "" {
			sig = fmt.Sprintf("%s where %s", fn.Signature, fn.Constraint)
		}
		entries = append(entries, &DocEntry{
			Name:        fn.Name,
			Signature:   sig,
			Description: fn.Description,
			Example:     fn.Example,
			Category:    fn.Category,
		})
	}
	return entries
}

// ============================================================================
// Prelude (built-in, no import needed)
// ============================================================================

func initPreludeDocs() {
	pkg := &DocPackage{
		Path:        "prelude",
		Description: "Built-in functions, types, operators, and traits (always available)",
		Types:       generateTypeDocs(),
		Traits:      generateTraitDocs(),
		Operators:   generateOperatorDocs(),
		Functions:   generateFunctionDocs(),
	}
	RegisterDocPackage(pkg)
}

// ============================================================================
// lib/list
// ============================================================================

func initListDocs() {
	meta := map[string]*DocMeta{
		// Access
		"head":   {Description: "First element (panics if empty)", Category: "Access"},
		"headOr": {Description: "First element or default", Category: "Access"},
		"last":   {Description: "Last element (panics if empty)", Category: "Access"},
		"lastOr": {Description: "Last element or default", Category: "Access"},
		"nth":    {Description: "Element at index (panics if out of bounds)", Category: "Access"},
		"nthOr":  {Description: "Element at index or default", Category: "Access"},
		// Sublist
		"tail":      {Description: "All elements except first", Category: "Sublist"},
		"init":      {Description: "All elements except last", Category: "Sublist"},
		"take":      {Description: "Take first n elements", Category: "Sublist"},
		"drop":      {Description: "Drop first n elements", Category: "Sublist"},
		"slice":     {Description: "Slice from start to end index", Category: "Sublist"},
		"takeWhile": {Description: "Take while predicate holds", Category: "Sublist"},
		"dropWhile": {Description: "Drop while predicate holds", Category: "Sublist"},
		// Predicates
		"length":   {Description: "Number of elements", Category: "Predicate"},
		"contains": {Description: "Check if element exists", Category: "Predicate"},
		"any":      {Description: "Any element matches predicate", Category: "Predicate"},
		"all":      {Description: "All elements match predicate", Category: "Predicate"},
		// Search
		"indexOf":   {Description: "Find index of element", Category: "Search"},
		"find":      {Description: "Find first matching element", Category: "Search"},
		"findIndex": {Description: "Find index of first match", Category: "Search"},
		// Higher-Order
		"map":       {Description: "Apply function to each element", Category: "Higher-Order"},
		"filter":    {Description: "Keep elements matching predicate", Category: "Higher-Order"},
		"foldl":     {Description: "Left fold with initial value", Category: "Higher-Order"},
		"foldr":     {Description: "Right fold with initial value", Category: "Higher-Order"},
		"partition": {Description: "Split by predicate into (matching, non-matching)", Category: "Higher-Order"},
		"forEach":   {Description: "Apply function to each element for side effects", Category: "Higher-Order"},
		// Transform
		"reverse": {Description: "Reverse list", Category: "Transform"},
		"concat":  {Description: "Flatten one level of nesting", Category: "Transform"},
		"flatten": {Description: "Flatten one level (alias for concat)", Category: "Transform"},
		"unique":  {Description: "Remove duplicates", Category: "Transform"},
		"sort":    {Description: "Sort elements (requires Order)", Category: "Transform"},
		"sortBy":  {Description: "Sort with custom comparator", Category: "Transform"},
		// Combining
		"zip":   {Description: "Pair elements from two lists", Category: "Combining"},
		"unzip": {Description: "Separate paired list", Category: "Combining"},
		"range": {Description: "Generate range [start, end)", Category: "Generation"},
	}
	pkg := generatePackageDocs("lib/list", "List manipulation functions", meta, nil)
	RegisterDocPackage(pkg)
}

// ============================================================================
// lib/map
// ============================================================================

func initMapDocs() {
	meta := map[string]*DocMeta{
		// Access
		"mapGet":      {Description: "Get value for key (returns Option)", Category: "Access"},
		"mapGetOr":    {Description: "Get value for key or default", Category: "Access"},
		"mapContains": {Description: "Check if key exists", Category: "Access"},
		"mapSize":     {Description: "Number of entries", Category: "Access"},
		// Modification (immutable - returns new map)
		"mapPut":    {Description: "Add or update key-value pair", Category: "Modification"},
		"mapRemove": {Description: "Remove key", Category: "Modification"},
		"mapMerge":  {Description: "Merge two maps (second wins on conflict)", Category: "Modification"},
		// Iteration
		"mapKeys":   {Description: "Get all keys as List", Category: "Iteration"},
		"mapValues": {Description: "Get all values as List", Category: "Iteration"},
		"mapItems":  {Description: "Get all key-value pairs as List<(K,V)>", Category: "Iteration"},
	}
	pkg := generatePackageDocs("lib/map", "Immutable hash map (HAMT-based)", meta, nil)
	RegisterDocPackage(pkg)
}

// ============================================================================
// lib/bytes
// ============================================================================

func initBytesDocs() {
	meta := map[string]*DocMeta{
		// Creation
		"bytesNew":        {Description: "Create empty bytes", Category: "Creation"},
		"bytesFromString": {Description: "Create bytes from UTF-8 string", Category: "Creation"},
		"bytesFromList":   {Description: "Create bytes from List<Int> (0-255)", Category: "Creation"},
		"bytesFromHex":    {Description: "Parse hex string (returns Result)", Category: "Creation"},
		"bytesFromBin":    {Description: "Parse binary string (returns Result)", Category: "Creation"},
		"bytesFromOct":    {Description: "Parse octal string (returns Result)", Category: "Creation"},
		// Access
		"bytesSlice": {Description: "Extract slice", Category: "Access"},
		// Conversion
		"bytesToString": {Description: "Convert to UTF-8 string (returns Result)", Category: "Conversion"},
		"bytesToList":   {Description: "Convert to List<Int>", Category: "Conversion"},
		"bytesToHex":    {Description: "Convert to hex string", Category: "Conversion"},
		"bytesToBin":    {Description: "Convert to binary string", Category: "Conversion"},
		"bytesToOct":    {Description: "Convert to octal string", Category: "Conversion"},
		// Modification
		"bytesConcat": {Description: "Concatenate bytes", Category: "Modification"},
		// Numeric Encoding
		"bytesEncodeInt":    {Description: "Encode Int to bytes", Category: "Numeric"},
		"bytesDecodeInt":    {Description: "Decode bytes to Int", Category: "Numeric"},
		"bytesEncodeBigInt": {Description: "Encode BigInt to bytes", Category: "Numeric"},
		"bytesDecodeBigInt": {Description: "Decode bytes to BigInt (returns Result)", Category: "Numeric"},
		"bytesEncodeFloat":  {Description: "Encode Float to bytes (size 4 or 8)", Category: "Numeric"},
		"bytesDecodeFloat":  {Description: "Decode bytes to Float (returns Result)", Category: "Numeric"},
		// Search
		"bytesContains":   {Description: "Check if bytes contains subsequence", Category: "Search"},
		"bytesIndexOf":    {Description: "Find position of subsequence (returns Option)", Category: "Search"},
		"bytesStartsWith": {Description: "Check prefix", Category: "Search"},
		"bytesEndsWith":   {Description: "Check suffix", Category: "Search"},
		// Split/Join
		"bytesSplit": {Description: "Split by separator", Category: "Split/Join"},
		"bytesJoin":  {Description: "Join list of bytes with separator", Category: "Split/Join"},
	}
	pkg := generatePackageDocs("lib/bytes", "Byte sequence manipulation", meta, nil)
	RegisterDocPackage(pkg)
}

// ============================================================================
// lib/bits
// ============================================================================

func initBitsDocs() {
	meta := map[string]*DocMeta{
		// Creation
		"bitsNew":        {Description: "Create empty Bits", Category: "Creation"},
		"bitsFromBytes":  {Description: "Create Bits from Bytes", Category: "Creation"},
		"bitsFromBinary": {Description: "Parse binary string to Bits", Category: "Creation"},
		"bitsFromHex":    {Description: "Parse hex string to Bits", Category: "Creation"},
		"bitsFromOctal":  {Description: "Parse octal string to Bits", Category: "Creation"},

		// Conversion
		"bitsToBytes":  {Description: "Convert to Bytes (padding: low|high, default: low)", Category: "Conversion"},
		"bitsToBinary": {Description: "Convert to binary string", Category: "Conversion"},
		"bitsToHex":    {Description: "Convert to hex string", Category: "Conversion"},

		// Access
		"bitsSlice": {Description: "Extract bit range [start, end)", Category: "Access"},
		"bitsGet":   {Description: "Get bit at index (Some(0|1) or Zero)", Category: "Access"},

		// Modification
		"bitsConcat":   {Description: "Concatenate two Bits", Category: "Modification"},
		"bitsSet":      {Description: "Set bit at index to 0 or 1", Category: "Modification"},
		"bitsPadLeft":  {Description: "Pad with zeros on left to target length", Category: "Modification"},
		"bitsPadRight": {Description: "Pad with zeros on right to target length", Category: "Modification"},

		// Numeric operations
		"bitsAddInt":   {Description: "Append int (val, size, spec?=big). Spec: big|little|native[-signed]", Category: "Numeric"},
		"bitsAddFloat": {Description: "Append float as bits (size: 32 or 64)", Category: "Numeric"},

		// Pattern matching
		"bitsExtract": {Description: "Extract fields using spec list", Category: "Pattern Matching"},
		"bitsInt":     {Description: "Spec for integer field (name, size, endianness)", Category: "Pattern Matching"},
		"bitsBytes":   {Description: "Spec for bytes field (name, size)", Category: "Pattern Matching"},
		"bitsRest":    {Description: "Spec for remaining bits (name)", Category: "Pattern Matching"},
	}
	pkg := generatePackageDocs("lib/bits", "Bit sequence manipulation (non-byte-aligned)", meta, nil)
	RegisterDocPackage(pkg)
}

// ============================================================================
// lib/string
// ============================================================================

func initStringDocs() {
	meta := map[string]*DocMeta{
		"stringSplit":      {Description: "Split by delimiter", Category: "Split/Join"},
		"stringJoin":       {Description: "Join with delimiter", Category: "Split/Join"},
		"stringLines":      {Description: "Split by newlines", Category: "Split/Join"},
		"stringWords":      {Description: "Split by whitespace", Category: "Split/Join"},
		"stringTrim":       {Description: "Remove leading/trailing whitespace", Category: "Trim"},
		"stringTrimStart":  {Description: "Remove leading whitespace", Category: "Trim"},
		"stringTrimEnd":    {Description: "Remove trailing whitespace", Category: "Trim"},
		"stringToUpper":    {Description: "Convert to uppercase", Category: "Case"},
		"stringToLower":    {Description: "Convert to lowercase", Category: "Case"},
		"stringCapitalize": {Description: "Capitalize first letter", Category: "Case"},
		"stringReplace":    {Description: "Replace first occurrence", Category: "Replace"},
		"stringReplaceAll": {Description: "Replace all occurrences", Category: "Replace"},
		"stringStartsWith": {Description: "Check prefix", Category: "Search"},
		"stringEndsWith":   {Description: "Check suffix", Category: "Search"},
		"stringIndexOf":    {Description: "Find substring index", Category: "Search"},
		"stringRepeat":     {Description: "Repeat string n times", Category: "Other"},
		"stringPadLeft":    {Description: "Pad on left to length", Category: "Other"},
		"stringPadRight":   {Description: "Pad on right to length", Category: "Other"},
	}
	pkg := generatePackageDocs("lib/string", "String manipulation functions (String = List<Char>)", meta, nil)
	RegisterDocPackage(pkg)
}

// ============================================================================
// lib/time
// ============================================================================

func initTimeDocs() {
	meta := map[string]*DocMeta{
		"timeNow": {Description: "Current Unix timestamp (seconds)", Category: "Wall Clock"},
		"clockNs": {Description: "Monotonic clock in nanoseconds", Category: "Monotonic"},
		"clockMs": {Description: "Monotonic clock in milliseconds", Category: "Monotonic"},
		"sleep":   {Description: "Sleep for n seconds", Category: "Sleep"},
		"sleepMs": {Description: "Sleep for n milliseconds", Category: "Sleep"},
	}
	pkg := generatePackageDocs("lib/time", "Time and timing functions", meta, nil)
	RegisterDocPackage(pkg)
}

// ============================================================================
// lib/io
// ============================================================================

func initIODocs() {
	meta := map[string]*DocMeta{
		// Stdin
		"readLine": {Description: "Read line from stdin (Zero on EOF)", Category: "Stdin"},

		// File operations
		"fileRead":   {Description: "Read entire file", Category: "File Read"},
		"fileReadAt": {Description: "Read file slice (offset, length)", Category: "File Read"},
		"fileWrite":  {Description: "Write/overwrite file", Category: "File Write"},
		"fileAppend": {Description: "Append to file", Category: "File Write"},
		"fileExists": {Description: "Check if file exists", Category: "File Info"},
		"fileSize":   {Description: "Get file size in bytes", Category: "File Info"},
		"fileDelete": {Description: "Delete file", Category: "File Ops"},

		// Directory operations
		"dirCreate":    {Description: "Create directory", Category: "Directory"},
		"dirCreateAll": {Description: "Create directory with parents (mkdir -p)", Category: "Directory"},
		"dirRemove":    {Description: "Remove empty directory", Category: "Directory"},
		"dirRemoveAll": {Description: "Remove directory recursively", Category: "Directory"},
		"dirList":      {Description: "List directory contents", Category: "Directory"},
		"dirExists":    {Description: "Check if directory exists", Category: "Directory"},

		// Path type checks
		"isDir":  {Description: "Check if path is a directory", Category: "Path Check"},
		"isFile": {Description: "Check if path is a regular file", Category: "Path Check"},
	}
	pkg := generatePackageDocs("lib/io", "File and stream I/O", meta, nil)
	RegisterDocPackage(pkg)
}

// ============================================================================
// lib/sys
// ============================================================================

func initSysDocs() {
	meta := map[string]*DocMeta{
		"sysArgs": {Description: "Command line arguments"},
		"sysEnv":  {Description: "Get environment variable"},
		"sysExit": {Description: "Exit with status code"},
		"sysExec": {Description: "Execute external command"},
	}
	pkg := generatePackageDocs("lib/sys", "System interaction (sysArgs, sysEnv, sysExit, sysExec)", meta, nil)
	RegisterDocPackage(pkg)
}

// ============================================================================
// lib/tuple
// ============================================================================

func initTupleDocs() {
	meta := map[string]*DocMeta{
		"fst":         {Description: "First element of pair", Category: "Access"},
		"snd":         {Description: "Second element of pair", Category: "Access"},
		"tupleGet":    {Description: "Get element by index", Category: "Access"},
		"tupleSwap":   {Description: "Swap pair elements", Category: "Transform"},
		"tupleDup":    {Description: "Duplicate value into pair", Category: "Transform"},
		"mapFst":      {Description: "Map first element", Category: "Map"},
		"mapSnd":      {Description: "Map second element", Category: "Map"},
		"mapPair":     {Description: "Map both elements", Category: "Map"},
		"curry":       {Description: "Curry pair function", Category: "Curry"},
		"uncurry":     {Description: "Uncurry to pair function", Category: "Curry"},
		"tupleBoth":   {Description: "Check both elements", Category: "Predicate"},
		"tupleEither": {Description: "Check either element", Category: "Predicate"},
	}
	pkg := generatePackageDocs("lib/tuple", "Tuple manipulation functions", meta, nil)
	RegisterDocPackage(pkg)
}

// ============================================================================
// lib/math
// ============================================================================

func initMathDocs() {
	meta := map[string]*DocMeta{
		"abs":    {Description: "Absolute value (Float)", Category: "Basic"},
		"absInt": {Description: "Absolute value (Int)", Category: "Basic"},
		"sign":   {Description: "Sign: -1, 0, or 1", Category: "Basic"},
		"min":    {Description: "Minimum (Float)", Category: "Comparison"},
		"max":    {Description: "Maximum (Float)", Category: "Comparison"},
		"minInt": {Description: "Minimum (Int)", Category: "Comparison"},
		"maxInt": {Description: "Maximum (Int)", Category: "Comparison"},
		"clamp":  {Description: "Clamp value to range", Category: "Comparison"},
		"floor":  {Description: "Round down", Category: "Rounding"},
		"ceil":   {Description: "Round up", Category: "Rounding"},
		"round":  {Description: "Round to nearest", Category: "Rounding"},
		"trunc":  {Description: "Truncate toward zero", Category: "Rounding"},
		"sqrt":   {Description: "Square root", Category: "Power"},
		"cbrt":   {Description: "Cube root", Category: "Power"},
		"pow":    {Description: "Power (x^y)", Category: "Power"},
		"exp":    {Description: "e^x", Category: "Power"},
		"log":    {Description: "Natural logarithm", Category: "Power"},
		"log10":  {Description: "Base-10 logarithm", Category: "Power"},
		"log2":   {Description: "Base-2 logarithm", Category: "Power"},
		"sin":    {Description: "Sine", Category: "Trig"},
		"cos":    {Description: "Cosine", Category: "Trig"},
		"tan":    {Description: "Tangent", Category: "Trig"},
		"asin":   {Description: "Arcsine", Category: "Trig"},
		"acos":   {Description: "Arccosine", Category: "Trig"},
		"atan":   {Description: "Arctangent", Category: "Trig"},
		"atan2":  {Description: "Arctangent of y/x", Category: "Trig"},
		"sinh":   {Description: "Hyperbolic sine", Category: "Hyperbolic"},
		"cosh":   {Description: "Hyperbolic cosine", Category: "Hyperbolic"},
		"tanh":   {Description: "Hyperbolic tangent", Category: "Hyperbolic"},
		"pi":     {Description: "π constant", Category: "Constants"},
		"e":      {Description: "e constant", Category: "Constants"},
	}
	pkg := generatePackageDocs("lib/math", "Mathematical functions", meta, nil)
	RegisterDocPackage(pkg)
}

// ============================================================================
// lib/bignum
// ============================================================================

func initBignumDocs() {
	meta := map[string]*DocMeta{
		"bigIntNew":      {Description: "Create BigInt from string", Category: "BigInt"},
		"bigIntFromInt":  {Description: "Create BigInt from Int", Category: "BigInt"},
		"bigIntToString": {Description: "Convert BigInt to string", Category: "BigInt"},
		"bigIntToInt":    {Description: "Convert to Int (Zero if overflow)", Category: "BigInt"},
		"ratNew":         {Description: "Create Rational from BigInts", Category: "Rational"},
		"ratFromInt":     {Description: "Create Rational from Ints", Category: "Rational"},
		"ratNumer":       {Description: "Get numerator", Category: "Rational"},
		"ratDenom":       {Description: "Get denominator", Category: "Rational"},
		"ratToFloat":     {Description: "Convert to Float", Category: "Rational"},
		"ratToString":    {Description: "Convert to string (\"num/denom\")", Category: "Rational"},
	}
	pkg := generatePackageDocs("lib/bignum", "Arbitrary precision numbers (BigInt, Rational)", meta, nil)
	RegisterDocPackage(pkg)
}

// ============================================================================
// lib/char
// ============================================================================

func initCharDocs() {
	meta := map[string]*DocMeta{
		"charToCode":   {Description: "Unicode code point"},
		"charFromCode": {Description: "Character from code point"},
		"charIsUpper":  {Description: "Is uppercase letter"},
		"charIsLower":  {Description: "Is lowercase letter"},
		"charToUpper":  {Description: "Convert to uppercase"},
		"charToLower":  {Description: "Convert to lowercase"},
	}
	pkg := generatePackageDocs("lib/char", "Character functions", meta, nil)
	RegisterDocPackage(pkg)
}

// ============================================================================
// lib/json
// ============================================================================

func initJsonDocs() {
	meta := map[string]*DocMeta{
		"jsonEncode":    {Description: "Encode value to JSON string"},
		"jsonDecode":    {Description: "Decode JSON to typed value"},
		"jsonParse":     {Description: "Parse JSON to Json ADT"},
		"jsonFromValue": {Description: "Convert value to Json ADT"},
		"jsonGet":       {Description: "Get field from JObj"},
		"jsonKeys":      {Description: "Get keys from JObj"},
	}
	types := []*DocEntry{
		{Name: "Json", Signature: "JNull | JBool Bool | JNum Float | JStr String | JArr List<Json> | JObj List<(String, Json)>", Description: "JSON value ADT"},
	}
	pkg := generatePackageDocs("lib/json", "JSON encoding, decoding, and manipulation", meta, types)
	RegisterDocPackage(pkg)
}

// ============================================================================
// lib/crypto
// ============================================================================

func initCryptoDocs() {
	meta := map[string]*DocMeta{
		"base64Encode":      {Description: "Encode to Base64", Category: "Encoding"},
		"base64Decode":      {Description: "Decode from Base64", Category: "Encoding"},
		"hexEncode":         {Description: "Encode to hex string", Category: "Encoding"},
		"hexDecode":         {Description: "Decode from hex string", Category: "Encoding"},
		"urlEncode":         {Description: "URL-encode (percent encoding)", Category: "Encoding"},
		"urlDecode":         {Description: "URL-decode", Category: "Encoding"},
		"md5":               {Description: "MD5 hash (32 hex chars)", Category: "Hash"},
		"sha1":              {Description: "SHA1 hash (40 hex chars)", Category: "Hash"},
		"sha256":            {Description: "SHA256 hash (64 hex chars)", Category: "Hash"},
		"sha512":            {Description: "SHA512 hash (128 hex chars)", Category: "Hash"},
		"hmacSha256":        {Description: "HMAC-SHA256 signature", Category: "HMAC"},
		"hmacSha512":        {Description: "HMAC-SHA512 signature", Category: "HMAC"},
		"cryptoRandomBytes": {Description: "Cryptographically secure random bytes", Category: "Random"},
		"cryptoRandomHex":   {Description: "Cryptographically secure random hex string", Category: "Random"},
	}
	pkg := generatePackageDocs("lib/crypto", "Cryptographic hashing, encoding, and secure random functions", meta, nil)
	RegisterDocPackage(pkg)
}

// ============================================================================
// lib/regex
// ============================================================================

func initRegexDocs() {
	meta := map[string]*DocMeta{
		"regexMatch":      {Description: "Test if pattern matches string", Category: "Match"},
		"regexFind":       {Description: "Find first match", Category: "Match"},
		"regexFindAll":    {Description: "Find all matches", Category: "Match"},
		"regexCapture":    {Description: "Capture groups from first match", Category: "Capture"},
		"regexReplace":    {Description: "Replace first match", Category: "Replace"},
		"regexReplaceAll": {Description: "Replace all matches", Category: "Replace"},
		"regexSplit":      {Description: "Split string by pattern", Category: "Split"},
		"regexValidate":   {Description: "Validate regex syntax", Category: "Utility"},
	}
	pkg := generatePackageDocs("lib/regex", "Regular expression matching and manipulation", meta, nil)
	RegisterDocPackage(pkg)
}

// ============================================================================
// lib/http
// ============================================================================

func initHttpDocs() {
	meta := map[string]*DocMeta{
		"httpGet":        {Description: "HTTP GET request", Category: "Client"},
		"httpPost":       {Description: "HTTP POST with string body", Category: "Client"},
		"httpPostJson":   {Description: "HTTP POST with JSON body (auto-encodes)", Category: "Client"},
		"httpPut":        {Description: "HTTP PUT with string body", Category: "Client"},
		"httpDelete":     {Description: "HTTP DELETE request", Category: "Client"},
		"httpRequest":    {Description: "Full control HTTP request (body=\"\" and timeout=0 defaults)", Category: "Client"},
		"httpSetTimeout": {Description: "Set request timeout (milliseconds)", Category: "Config"},
		"httpServe":      {Description: "Start HTTP server (blocking)", Category: "Server"},
		"httpServeAsync": {Description: "Start HTTP server (non-blocking, returns server ID)", Category: "Server"},
		"httpServerStop": {Description: "Stop a running server by ID (timeout? default 5000ms)", Category: "Server"},
	}
	types := []*DocEntry{
		{Name: "HttpResponse", Signature: "{ status: Int, body: String, headers: List<(String, String)> }", Description: "HTTP response type"},
		{Name: "HttpRequest", Signature: "{ method: String, path: String, query: String, headers: List<(String, String)>, body: String }", Description: "HTTP request type (server)"},
	}
	pkg := generatePackageDocs("lib/http", "HTTP client and server", meta, types)
	RegisterDocPackage(pkg)
}

// ============================================================================
// lib/test
// ============================================================================

func initTestDocs() {
	meta := map[string]*DocMeta{
		// Test definition
		"testRun":        {Description: "Define and run a test with name and body", Category: "Test Definition"},
		"testSkip":       {Description: "Skip current test with reason", Category: "Test Definition"},
		"testExpectFail": {Description: "Test that is expected to fail (for known bugs)", Category: "Test Definition"},
		// Assertions
		"assert":       {Description: "Assert condition is true", Category: "Assertions"},
		"assertEquals": {Description: "Assert two values are equal", Category: "Assertions"},
		"assertOk":     {Description: "Assert Result is Ok", Category: "Assertions"},
		"assertFail":   {Description: "Assert Result is Fail", Category: "Assertions"},
		"assertSome":   {Description: "Assert Option is Some", Category: "Assertions"},
		"assertZero":   {Description: "Assert Option is Zero", Category: "Assertions"},
		// HTTP mocks
		"mockHttp":       {Description: "Mock HTTP response for URL pattern", Category: "HTTP Mocks"},
		"mockHttpError":  {Description: "Mock HTTP error for URL pattern", Category: "HTTP Mocks"},
		"mockHttpOff":    {Description: "Disable all HTTP mocks", Category: "HTTP Mocks"},
		"mockHttpBypass": {Description: "Bypass HTTP mocks for one call", Category: "HTTP Mocks"},
		// File mocks
		"mockFile":       {Description: "Mock file read for path pattern", Category: "File Mocks"},
		"mockFileOff":    {Description: "Disable all file mocks", Category: "File Mocks"},
		"mockFileBypass": {Description: "Bypass file mocks for one call", Category: "File Mocks"},
		// Env mocks
		"mockEnv":       {Description: "Mock environment variable", Category: "Env Mocks"},
		"mockEnvOff":    {Description: "Disable all env mocks", Category: "Env Mocks"},
		"mockEnvBypass": {Description: "Bypass env mocks for one call", Category: "Env Mocks"},
	}
	pkg := generatePackageDocs("lib/test", "Testing framework with assertions and mocking", meta, nil)
	RegisterDocPackage(pkg)
}

// ============================================================================
// lib/rand
// ============================================================================

func initRandDocs() {
	meta := map[string]*DocMeta{
		"randomInt":        {Description: "Random integer", Category: "Generation"},
		"randomIntRange":   {Description: "Random integer in [min, max]", Category: "Generation"},
		"randomFloat":      {Description: "Random float in [0.0, 1.0)", Category: "Generation"},
		"randomFloatRange": {Description: "Random float in [min, max)", Category: "Generation"},
		"randomBool":       {Description: "Random boolean", Category: "Generation"},
		"randomChoice":     {Description: "Random element from list", Category: "Lists"},
		"randomShuffle":    {Description: "Shuffle list randomly", Category: "Lists"},
		"randomSample":     {Description: "Random sample of n elements", Category: "Lists"},
		"randomSeed":       {Description: "Set random seed for reproducibility", Category: "Configuration"},
	}
	pkg := generatePackageDocs("lib/rand", "Random number generation", meta, nil)
	RegisterDocPackage(pkg)
}

// ============================================================================
// lib/date
// ============================================================================

func initDateDocs() {
	meta := map[string]*DocMeta{
		// Creation
		"dateNow":           {Description: "Current local date/time (with local offset)", Category: "Creation"},
		"dateNowUtc":        {Description: "Current UTC date/time (offset=0)", Category: "Creation"},
		"dateFromTimestamp": {Description: "Date from Unix timestamp (local offset)", Category: "Creation"},
		"dateNew":           {Description: "Create date (y, m, d, offset=local)", Category: "Creation"},
		"dateNewTime":       {Description: "Create date (y, m, d, h, min, s, offset=local)", Category: "Creation"},
		"dateToTimestamp":   {Description: "Convert to Unix timestamp", Category: "Creation"},

		// Timezone/Offset
		"dateToUtc":      {Description: "Convert to UTC (offset=0)", Category: "Offset"},
		"dateToLocal":    {Description: "Convert to local time", Category: "Offset"},
		"dateOffset":     {Description: "Get offset in minutes from UTC", Category: "Offset"},
		"dateWithOffset": {Description: "Change offset (adjusts time)", Category: "Offset"},

		// Formatting
		"dateFormat": {Description: "Format date to string", Category: "Formatting"},
		"dateParse":  {Description: "Parse string to date", Category: "Formatting"},

		// Components
		"dateYear":    {Description: "Get year component", Category: "Components"},
		"dateMonth":   {Description: "Get month component (1-12)", Category: "Components"},
		"dateDay":     {Description: "Get day component (1-31)", Category: "Components"},
		"dateWeekday": {Description: "Get weekday (0=Sun, 6=Sat)", Category: "Components"},
		"dateHour":    {Description: "Get hour component (0-23)", Category: "Components"},
		"dateMinute":  {Description: "Get minute component (0-59)", Category: "Components"},
		"dateSecond":  {Description: "Get second component (0-59)", Category: "Components"},

		// Arithmetic
		"dateAddDays":    {Description: "Add/subtract days", Category: "Arithmetic"},
		"dateAddMonths":  {Description: "Add/subtract months", Category: "Arithmetic"},
		"dateAddYears":   {Description: "Add/subtract years", Category: "Arithmetic"},
		"dateAddHours":   {Description: "Add/subtract hours", Category: "Arithmetic"},
		"dateAddMinutes": {Description: "Add/subtract minutes", Category: "Arithmetic"},
		"dateAddSeconds": {Description: "Add/subtract seconds", Category: "Arithmetic"},

		// Difference
		"dateDiffDays":    {Description: "Difference in days", Category: "Difference"},
		"dateDiffSeconds": {Description: "Difference in seconds", Category: "Difference"},
	}
	types := []*DocEntry{
		{Name: "Date", Signature: "{ year, month, day, hour, minute, second, offset: Int }", Description: "Date/time record with offset (minutes from UTC)"},
	}
	pkg := generatePackageDocs("lib/date", "Date and time manipulation with timezone offset", meta, types)
	RegisterDocPackage(pkg)
}

// ============================================================================
// lib/ws (WebSocket)
// ============================================================================

func initWsDocs() {
	meta := map[string]*DocMeta{
		// Client
		"wsConnect":        {Description: "Connect to WebSocket server (30s timeout)", Category: "Client"},
		"wsConnectTimeout": {Description: "Connect with custom timeout (ms)", Category: "Client"},
		"wsSend":           {Description: "Send text message", Category: "Client"},
		"wsRecv":           {Description: "Receive message (blocking)", Category: "Client"},
		"wsRecvTimeout":    {Description: "Receive with timeout (returns Option)", Category: "Client"},
		"wsClose":          {Description: "Close connection", Category: "Client"},

		// Server
		"wsServe":      {Description: "Start blocking WebSocket server", Category: "Server"},
		"wsServeAsync": {Description: "Start non-blocking server (returns ID)", Category: "Server"},
		"wsServerStop": {Description: "Stop async server by ID", Category: "Server"},
	}
	pkg := generatePackageDocs("lib/ws", "WebSocket client and server (RFC 6455)", meta, nil)
	RegisterDocPackage(pkg)
}

// ============================================================================
// lib/sql (SQLite Database)
// ============================================================================

func initSqlDocs() {
	meta := map[string]*DocMeta{
		// Connection
		"sqlOpen":  {Description: "Open database connection (driver, dsn) -> Result<String, SqlDB>", Category: "Connection"},
		"sqlClose": {Description: "Close connection", Category: "Connection"},
		"sqlPing":  {Description: "Test connection", Category: "Connection"},

		// Query
		"sqlQuery":        {Description: "Execute SELECT, returns List<Row>", Category: "Query"},
		"sqlQueryRow":     {Description: "Execute SELECT, returns Option<Row>", Category: "Query"},
		"sqlExec":         {Description: "Execute INSERT/UPDATE/DELETE, returns affected rows", Category: "Query"},
		"sqlLastInsertId": {Description: "Execute INSERT, returns last insert ID", Category: "Query"},

		// Transaction
		"sqlBegin":    {Description: "Start transaction", Category: "Transaction"},
		"sqlCommit":   {Description: "Commit transaction", Category: "Transaction"},
		"sqlRollback": {Description: "Rollback transaction", Category: "Transaction"},
		"sqlTxQuery":  {Description: "Query in transaction", Category: "Transaction"},
		"sqlTxExec":   {Description: "Execute in transaction", Category: "Transaction"},

		// Utility
		"sqlUnwrap": {Description: "Extract value from SqlValue -> Option<T>", Category: "Utility"},
		"sqlIsNull": {Description: "Check if SqlNull -> Bool", Category: "Utility"},
	}

	types := []*DocEntry{
		{Name: "SqlValue", Signature: "SqlNull | SqlInt Int | SqlFloat Float | SqlString String | SqlBool Bool | SqlBytes Bytes | SqlTime Date | SqlBigInt BigInt", Description: "SQL value ADT"},
		{Name: "SqlDB", Signature: "opaque", Description: "Database connection handle"},
		{Name: "SqlTx", Signature: "opaque", Description: "Transaction handle"},
		{Name: "Row", Signature: "Map<String, SqlValue>", Description: "Query result row"},
	}

	pkg := generatePackageDocs("lib/sql", "SQLite database operations", meta, types)
	RegisterDocPackage(pkg)
}

// ============================================================================
// lib/url documentation
// ============================================================================

func initUrlDocs() {
	meta := map[string]*DocMeta{
		// Parsing
		"urlParse":    {Description: "Parse URL string", Category: "Parsing"},
		"urlToString": {Description: "Convert URL to string", Category: "Parsing"},

		// Accessors
		"urlScheme":   {Description: "Get URL scheme", Category: "Accessors"},
		"urlUserinfo": {Description: "Get userinfo (user:pass)", Category: "Accessors"},
		"urlHost":     {Description: "Get host", Category: "Accessors"},
		"urlPort":     {Description: "Get port", Category: "Accessors"},
		"urlPath":     {Description: "Get path", Category: "Accessors"},
		"urlQuery":    {Description: "Get raw query string", Category: "Accessors"},
		"urlFragment": {Description: "Get fragment", Category: "Accessors"},

		// Query params
		"urlQueryParams":   {Description: "Get all query params as map", Category: "Query Params"},
		"urlQueryParam":    {Description: "Get first value for query param", Category: "Query Params"},
		"urlQueryParamAll": {Description: "Get all values for query param", Category: "Query Params"},

		// Modifiers
		"urlWithScheme":    {Description: "Set scheme", Category: "Modifiers"},
		"urlWithUserinfo":  {Description: "Set userinfo", Category: "Modifiers"},
		"urlWithHost":      {Description: "Set host", Category: "Modifiers"},
		"urlWithPort":      {Description: "Set port", Category: "Modifiers"},
		"urlWithPath":      {Description: "Set path", Category: "Modifiers"},
		"urlWithQuery":     {Description: "Set query", Category: "Modifiers"},
		"urlWithFragment":  {Description: "Set fragment", Category: "Modifiers"},
		"urlAddQueryParam": {Description: "Add query parameter", Category: "Modifiers"},

		// Utility
		"urlJoin":   {Description: "Join base URL with relative path", Category: "Utility"},
		"urlEncode": {Description: "URL encode (percent encoding)", Category: "Utility"},
		"urlDecode": {Description: "URL decode", Category: "Utility"},
	}

	types := []*DocEntry{
		{Name: "Url", Signature: "{ scheme, userinfo, host, port: Option<Int>, path, query, fragment }", Description: "URL record type"},
	}

	pkg := generatePackageDocs("lib/url", "URL parsing, manipulation, and encoding", meta, types)
	RegisterDocPackage(pkg)
}

// ============================================================================
// lib/path documentation
// ============================================================================

func initPathDocs() {
	meta := map[string]*DocMeta{
		// Parsing
		"pathJoin":  {Description: "Join path parts into single path", Category: "Parsing"},
		"pathSplit": {Description: "Split path into parts", Category: "Parsing"},
		"pathDir":   {Description: "Get directory part of path", Category: "Parsing"},
		"pathBase":  {Description: "Get file name part of path", Category: "Parsing"},
		"pathExt":   {Description: "Get file extension", Category: "Parsing"},
		"pathStem":  {Description: "Get file name without extension", Category: "Parsing"},

		// Manipulation
		"pathWithExt":  {Description: "Replace file extension", Category: "Manipulation"},
		"pathWithBase": {Description: "Replace file name", Category: "Manipulation"},

		// Query
		"pathIsAbs": {Description: "Check if path is absolute", Category: "Query"},
		"pathIsRel": {Description: "Check if path is relative", Category: "Query"},

		// Normalization
		"pathClean": {Description: "Clean path (remove .., .)", Category: "Normalization"},
		"pathAbs":   {Description: "Get absolute path", Category: "Normalization"},
		"pathRel":   {Description: "Get relative path from base to target", Category: "Normalization"},

		// Matching
		"pathMatch": {Description: "Match path against glob pattern", Category: "Matching"},

		// Separator
		"pathSep": {Description: "Get OS path separator", Category: "Other"},

		// Temp directory
		"pathTemp": {Description: "Get OS temp directory path", Category: "Other"},

		// POSIX-style
		"pathExtPosix":  {Description: "Get extension (POSIX-style, handles dotfiles)", Category: "POSIX"},
		"pathStemPosix": {Description: "Get stem (POSIX-style, handles dotfiles)", Category: "POSIX"},
		"pathIsHidden":  {Description: "Check if file is hidden (starts with .)", Category: "POSIX"},
	}

	pkg := generatePackageDocs("lib/path", "File path manipulation (OS-specific)", meta, nil)
	RegisterDocPackage(pkg)
}

// ============================================================================
// lib/uuid Documentation
// ============================================================================

func initUuidDocs() {
	meta := map[string]*DocMeta{
		// Generation
		"uuidNew": {Description: "Generate random UUID (v4)", Category: "Generation"},
		"uuidV4":  {Description: "Alias for uuidNew", Category: "Generation"},
		"uuidV5":  {Description: "Generate deterministic UUID from namespace + name (SHA-1)", Category: "Generation"},
		"uuidV7":  {Description: "Generate time-ordered UUID", Category: "Generation"},
		"uuidNil": {Description: "Get nil UUID (all zeros)", Category: "Generation"},
		"uuidMax": {Description: "Get max UUID (all ones)", Category: "Generation"},

		// Namespaces
		"uuidNamespaceDNS":  {Description: "DNS namespace UUID for v5", Category: "Namespaces"},
		"uuidNamespaceURL":  {Description: "URL namespace UUID for v5", Category: "Namespaces"},
		"uuidNamespaceOID":  {Description: "OID namespace UUID for v5", Category: "Namespaces"},
		"uuidNamespaceX500": {Description: "X.500 namespace UUID for v5", Category: "Namespaces"},

		// Parsing
		"uuidParse":     {Description: "Parse UUID from string", Category: "Parsing"},
		"uuidFromBytes": {Description: "Create UUID from 16 bytes", Category: "Parsing"},

		// Conversion
		"uuidToString":        {Description: "Convert to standard format (8-4-4-4-12)", Category: "Conversion"},
		"uuidToStringCompact": {Description: "Convert to compact format (no dashes)", Category: "Conversion"},
		"uuidToStringUrn":     {Description: "Convert to URN format (urn:uuid:...)", Category: "Conversion"},
		"uuidToStringBraces":  {Description: "Convert to braces format ({...})", Category: "Conversion"},
		"uuidToStringUpper":   {Description: "Convert to uppercase", Category: "Conversion"},
		"uuidToBytes":         {Description: "Convert to 16 bytes", Category: "Conversion"},

		// Info
		"uuidVersion": {Description: "Get UUID version (1, 4, 5, 7)", Category: "Info"},
		"uuidIsNil":   {Description: "Check if UUID is nil", Category: "Info"},
	}

	types := []*DocEntry{
		{Name: "Uuid", Signature: "opaque", Description: "UUID value (128-bit identifier)"},
	}

	pkg := generatePackageDocs("lib/uuid", "UUID generation and manipulation", meta, types)
	RegisterDocPackage(pkg)
}

// ============================================================================
// lib/log Documentation
// ============================================================================

func initLogDocs() {
	meta := map[string]*DocMeta{
		// Basic logging
		"logDebug":     {Description: "Log debug message", Category: "Basic"},
		"logInfo":      {Description: "Log info message", Category: "Basic"},
		"logWarn":      {Description: "Log warning message", Category: "Basic"},
		"logError":     {Description: "Log error message", Category: "Basic"},
		"logFatal":     {Description: "Log fatal message (no exit)", Category: "Basic"},
		"logFatalExit": {Description: "Log fatal message and exit(1)", Category: "Basic"},

		// Configuration
		"logLevel":  {Description: "Set minimum log level (debug|info|warn|error|fatal)", Category: "Configuration"},
		"logFormat": {Description: "Set output format (text|json)", Category: "Configuration"},
		"logOutput": {Description: "Set output destination (stderr|stdout|filepath)", Category: "Configuration"},
		"logColor":  {Description: "Enable/disable ANSI colors", Category: "Configuration"},

		// Structured
		"logWithFields": {Description: "Log with structured fields", Category: "Structured"},

		// Prefixed
		"logWithPrefix":    {Description: "Create logger with prefix", Category: "Prefixed"},
		"loggerDebug":      {Description: "Debug on prefixed logger", Category: "Prefixed"},
		"loggerInfo":       {Description: "Info on prefixed logger", Category: "Prefixed"},
		"loggerWarn":       {Description: "Warn on prefixed logger", Category: "Prefixed"},
		"loggerError":      {Description: "Error on prefixed logger", Category: "Prefixed"},
		"loggerFatal":      {Description: "Fatal on prefixed logger", Category: "Prefixed"},
		"loggerFatalExit":  {Description: "Fatal + exit on prefixed logger", Category: "Prefixed"},
		"loggerWithFields": {Description: "Structured log on prefixed logger", Category: "Prefixed"},
	}

	types := []*DocEntry{
		{Name: "Logger", Signature: "opaque", Description: "Prefixed logger instance"},
	}

	pkg := generatePackageDocs("lib/log", "Structured logging with levels, formats, and prefixed loggers", meta, types)
	RegisterDocPackage(pkg)
}

// ============================================================================
// lib/task Documentation
// ============================================================================

func initTaskDocs() {
	meta := map[string]*DocMeta{
		// Creation
		"async":       {Description: "Start async computation", Category: "Creation"},
		"taskResolve": {Description: "Create completed task with value", Category: "Creation"},
		"taskReject":  {Description: "Create failed task with error", Category: "Creation"},

		// Awaiting
		"await":             {Description: "Wait for task completion", Category: "Awaiting"},
		"awaitTimeout":      {Description: "Wait with timeout (ms)", Category: "Awaiting"},
		"awaitAll":          {Description: "Wait for all tasks to complete", Category: "Awaiting"},
		"awaitAllTimeout":   {Description: "Wait for all tasks with timeout", Category: "Awaiting"},
		"awaitAny":          {Description: "Wait for first successful task", Category: "Awaiting"},
		"awaitAnyTimeout":   {Description: "Wait for first success with timeout", Category: "Awaiting"},
		"awaitFirst":        {Description: "Wait for first completed (success or fail)", Category: "Awaiting"},
		"awaitFirstTimeout": {Description: "Wait for first completed with timeout", Category: "Awaiting"},

		// Control
		"taskCancel":      {Description: "Cancel task (if not started)", Category: "Control"},
		"taskIsDone":      {Description: "Check if task completed", Category: "Control"},
		"taskIsCancelled": {Description: "Check if task was cancelled", Category: "Control"},

		// Pool
		"taskSetGlobalPool": {Description: "Set maximum concurrent tasks", Category: "Pool"},
		"taskGetGlobalPool": {Description: "Get current pool limit", Category: "Pool"},

		// Combinators
		"taskMap":     {Description: "Transform task result", Category: "Combinators"},
		"taskFlatMap": {Description: "Chain tasks (monadic bind)", Category: "Combinators"},
		"taskCatch":   {Description: "Recover from task error", Category: "Combinators"},
	}

	types := []*DocEntry{
		{Name: "Task<T>", Signature: "opaque", Description: "Asynchronous computation that yields T"},
	}

	pkg := generatePackageDocs("lib/task", "Asynchronous computations with Tasks (Futures/Promises)", meta, types)
	RegisterDocPackage(pkg)
}

// ============================================================================
// lib/csv Documentation
// ============================================================================

func initCsvDocs() {
	meta := map[string]*DocMeta{
		// Parse
		"csvParse":    {Description: "Parse CSV with headers (delimiter? default ',')", Category: "Parse"},
		"csvParseRaw": {Description: "Parse CSV without headers (delimiter?)", Category: "Parse"},

		// Read
		"csvRead":    {Description: "Read CSV file with headers (delimiter?)", Category: "Read"},
		"csvReadRaw": {Description: "Read CSV file without headers (delimiter?)", Category: "Read"},

		// Encode
		"csvEncode":    {Description: "Encode records to CSV string (delimiter?)", Category: "Encode"},
		"csvEncodeRaw": {Description: "Encode rows to CSV string (delimiter?)", Category: "Encode"},

		// Write
		"csvWrite":    {Description: "Write records to CSV file (delimiter?)", Category: "Write"},
		"csvWriteRaw": {Description: "Write rows to CSV file (delimiter?)", Category: "Write"},
	}

	pkg := generatePackageDocs("lib/csv", "CSV parsing, encoding, and file I/O (optional delimiter, default ',')", meta, nil)
	RegisterDocPackage(pkg)
}

// ============================================================================
// lib/flag
// ============================================================================

func initFlagDocs() {
	meta := map[string]*DocMeta{
		"flagSet":   {Description: "Define a flag with default value and description", Category: "Definition"},
		"flagParse": {Description: "Parse command line arguments. Supports -flag=val, -flag val, --flag val", Category: "Parsing"},
		"flagGet":   {Description: "Get flag value (returns default if not set)", Category: "Access"},
		"flagArgs":  {Description: "Get remaining non-flag arguments", Category: "Access"},
		"flagUsage": {Description: "Print usage information", Category: "Help"},
	}
	pkg := generatePackageDocs("lib/flag", "Command line flag parsing. Supports both -flag=value and -flag value formats.", meta, nil)
	RegisterDocPackage(pkg)
}
