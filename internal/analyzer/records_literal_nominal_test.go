package analyzer

import (
	"github.com/funvibe/funxy/internal/diagnostics"
	"testing"
)

// These tests guard the nominal-type preservation for record literals
// (analyzer/inference_literals.go). When an inline record literal is expected
// to have a nominal record type (e.g. a `type alias Item = {...}` field filled
// via spread), the analyzer stamps the literal with the nominal type so the VM
// can preserve its identity for downstream pattern matching.
//
// Critically, adopting the nominal tag must NOT bypass structural validation:
// a literal with a wrong field type, a missing required field, or an extra
// field must still be rejected at compile time (otherwise it fails at runtime,
// e.g. "record has no field 'fails'"). Validation is strict, consistent with
// how record function returns are checked (statements.go): if the nominal path
// silently allowed extra fields, they would be swallowed by the nominal tag and
// hidden from the outer strict return check, making `{item: Item}` and
// `{item: Item | ...}` contexts behave inconsistently.

const recordsLiteralNominalPrelude = `
type alias Item = { target: Int, weight: Int, fails: Int }
type alias Box = { item: Item }
`

// TestNominalRecordLiteral_ValidSpreadUpdate ensures a correct inline record
// literal in a spread update is accepted and keeps its nominal identity.
func TestNominalRecordLiteral_ValidSpreadUpdate(t *testing.T) {
	input := recordsLiteralNominalPrelude + `
fun bump(box: Box) -> Box {
    {...box, item: {target: box.item.target, weight: box.item.weight, fails: box.item.fails + 1}}
}`
	expectNoAnalyzerErrors(t, input)
}

// TestNominalRecordLiteral_WrongFieldType rejects a literal whose field type
// does not match the nominal record's declared field type.
func TestNominalRecordLiteral_WrongFieldType(t *testing.T) {
	input := recordsLiteralNominalPrelude + `
fun bump(box: Box) -> Box {
    {...box, item: {target: box.item.target, weight: "oops", fails: box.item.fails}}
}`
	expectAnalyzerError(t, input, diagnostics.ErrA003)
}

// TestNominalRecordLiteral_MissingField rejects a literal that omits a required
// field of the nominal record type.
func TestNominalRecordLiteral_MissingField(t *testing.T) {
	input := recordsLiteralNominalPrelude + `
fun bump(box: Box) -> Box {
    {...box, item: {target: box.item.target, weight: box.item.weight}}
}`
	expectAnalyzerError(t, input, diagnostics.ErrA003)
}

// TestNominalRecordLiteral_ExtraFieldRejected ensures the nominal path applies
// strict validation: a literal with an extra field is rejected where a nominal
// record type is expected, matching the strict record function-return check.
func TestNominalRecordLiteral_ExtraFieldRejected(t *testing.T) {
	input := recordsLiteralNominalPrelude + `
fun bump(box: Box) -> Box {
    {...box, item: {target: box.item.target, weight: box.item.weight, fails: box.item.fails, extra: 1}}
}`
	expectAnalyzerError(t, input, diagnostics.ErrA003)
}

func TestNominalRecordLiteral_SelectsMatchingUnionMember(t *testing.T) {
	input := `
type alias A = { a: Int }
type alias B = { b: Int }
type alias Holder = { item: A | B }

fun replace(h: Holder) -> Holder {
    {...h, item: {b: 1}}
}`
	expectNoAnalyzerErrors(t, input)
}

func TestNominalRecordLiteral_PreservesGenericAlias(t *testing.T) {
	input := `
type alias Entry<t> = { value: t }
type alias Holder = { item: Entry<Int> }

fun replace(h: Holder) -> Holder {
    {...h, item: {value: 1}}
}`
	expectNoAnalyzerErrors(t, input)
}
