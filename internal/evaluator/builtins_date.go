package evaluator

import (
	"github.com/funvibe/funxy/internal/typesystem"
	"time"
)

// Get local offset in minutes
func getLocalOffset() int64 {
	_, offset := time.Now().Zone()
	return int64(offset / 60) // Convert seconds to minutes
}

// DateBuiltins returns built-in functions for lib/date virtual package
func DateBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		// Creation
		"dateNow":           {Fn: builtinDateNow, Name: "dateNow"},
		"dateNowUtc":        {Fn: builtinDateNowUtc, Name: "dateNowUtc"},
		"dateFromTimestamp": {Fn: builtinDateFromTimestamp, Name: "dateFromTimestamp"},
		"dateNew":           {Fn: builtinDateNew, Name: "dateNew"},
		"dateNewTime":       {Fn: builtinDateNewTime, Name: "dateNewTime"},
		"dateToTimestamp":   {Fn: builtinDateToTimestamp, Name: "dateToTimestamp"},

		// Timezone/Offset
		"dateToUtc":      {Fn: builtinDateToUtc, Name: "dateToUtc"},
		"dateToLocal":    {Fn: builtinDateToLocal, Name: "dateToLocal"},
		"dateOffset":     {Fn: builtinDateOffset, Name: "dateOffset"},
		"dateWithOffset": {Fn: builtinDateWithOffset, Name: "dateWithOffset"},

		// Formatting
		"dateFormat": {Fn: builtinDateFormat, Name: "dateFormat"},
		"dateParse":  {Fn: builtinDateParse, Name: "dateParse"},

		// Components
		"dateYear":    {Fn: builtinDateYear, Name: "dateYear"},
		"dateMonth":   {Fn: builtinDateMonth, Name: "dateMonth"},
		"dateDay":     {Fn: builtinDateDay, Name: "dateDay"},
		"dateWeekday": {Fn: builtinDateWeekday, Name: "dateWeekday"},
		"dateHour":    {Fn: builtinDateHour, Name: "dateHour"},
		"dateMinute":  {Fn: builtinDateMinute, Name: "dateMinute"},
		"dateSecond":  {Fn: builtinDateSecond, Name: "dateSecond"},

		// Arithmetic
		"dateAddDays":    {Fn: builtinDateAddDays, Name: "dateAddDays"},
		"dateAddMonths":  {Fn: builtinDateAddMonths, Name: "dateAddMonths"},
		"dateAddYears":   {Fn: builtinDateAddYears, Name: "dateAddYears"},
		"dateAddHours":   {Fn: builtinDateAddHours, Name: "dateAddHours"},
		"dateAddMinutes": {Fn: builtinDateAddMinutes, Name: "dateAddMinutes"},
		"dateAddSeconds": {Fn: builtinDateAddSeconds, Name: "dateAddSeconds"},

		// Difference
		"dateDiffDays":    {Fn: builtinDateDiffDays, Name: "dateDiffDays"},
		"dateDiffSeconds": {Fn: builtinDateDiffSeconds, Name: "dateDiffSeconds"},
	}
}

// RegisterDateBuiltins registers Date types and functions into an environment
func RegisterDateBuiltins(env *Environment) {
	// Date = { year: Int, month: Int, day: Int, hour: Int, minute: Int, second: Int, offset: Int }
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
	env.Set("Date", &TypeObject{TypeVal: dateType})

	// Functions
	builtins := DateBuiltins()
	SetDateBuiltinTypes(builtins)
	for name, fn := range builtins {
		env.Set(name, fn)
	}
}

// Date record type: { year, month, day, hour, minute, second, offset }
// offset is in minutes from UTC (e.g., 180 = UTC+3, -300 = UTC-5)
func makeDateWithOffset(t time.Time, offsetMinutes int64) *RecordInstance {
	return NewRecord(map[string]Object{
		"year":   &Integer{Value: int64(t.Year())},
		"month":  &Integer{Value: int64(t.Month())},
		"day":    &Integer{Value: int64(t.Day())},
		"hour":   &Integer{Value: int64(t.Hour())},
		"minute": &Integer{Value: int64(t.Minute())},
		"second": &Integer{Value: int64(t.Second())},
		"offset": &Integer{Value: offsetMinutes},
	})
}

// makeDate creates a date with the time's own offset
func makeDate(t time.Time) *RecordInstance {
	_, offset := t.Zone()
	return makeDateWithOffset(t, int64(offset/60))
}

// dateToTime converts a Date record to time.Time using its offset
func dateToTime(date *RecordInstance) (time.Time, bool) {
	yearObj := date.Get("year")
	if yearObj == nil {
		return time.Time{}, false
	}
	year, ok := yearObj.(*Integer)
	if !ok {
		return time.Time{}, false
	}

	monthObj := date.Get("month")
	if monthObj == nil {
		return time.Time{}, false
	}
	month, ok := monthObj.(*Integer)
	if !ok {
		return time.Time{}, false
	}

	dayObj := date.Get("day")
	if dayObj == nil {
		return time.Time{}, false
	}
	day, ok := dayObj.(*Integer)
	if !ok {
		return time.Time{}, false
	}

	hourObj := date.Get("hour")
	if hourObj == nil {
		return time.Time{}, false
	}
	hour, ok := hourObj.(*Integer)
	if !ok {
		return time.Time{}, false
	}

	minuteObj := date.Get("minute")
	if minuteObj == nil {
		return time.Time{}, false
	}
	minute, ok := minuteObj.(*Integer)
	if !ok {
		return time.Time{}, false
	}

	secondObj := date.Get("second")
	if secondObj == nil {
		return time.Time{}, false
	}
	second, ok := secondObj.(*Integer)
	if !ok {
		return time.Time{}, false
	}

	// Get offset (default to local if not present for backward compatibility)
	offsetMinutes := getLocalOffset()
	if offsetObj := date.Get("offset"); offsetObj != nil {
		if offset, ok := offsetObj.(*Integer); ok {
			offsetMinutes = offset.Value
		}
	}

	// Create location from offset
	loc := time.FixedZone("", int(offsetMinutes)*60)

	return time.Date(
		int(year.Value),
		time.Month(month.Value),
		int(day.Value),
		int(hour.Value),
		int(minute.Value),
		int(second.Value),
		0, loc,
	), true
}

// getDateOffset extracts offset from date record
func getDateOffset(date *RecordInstance) int64 {
	if offsetObj := date.Get("offset"); offsetObj != nil {
		if offset, ok := offsetObj.(*Integer); ok {
			return offset.Value
		}
	}
	return getLocalOffset()
}

// dateNow: () -> Date (with local offset)
func builtinDateNow(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("dateNow expects 0 arguments, got %d", len(args))
	}
	return makeDate(time.Now())
}

// dateNowUtc: () -> Date (with offset=0)
func builtinDateNowUtc(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("dateNowUtc expects 0 arguments, got %d", len(args))
	}
	return makeDateWithOffset(time.Now().UTC(), 0)
}

// dateFromTimestamp: (Int) -> Date (with local offset)
func builtinDateFromTimestamp(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("dateFromTimestamp expects 1 argument, got %d", len(args))
	}

	tsInt, ok := args[0].(*Integer)
	if !ok {
		return newError("dateFromTimestamp expects an integer, got %s", args[0].Type())
	}

	t := time.Unix(tsInt.Value, 0)
	return makeDate(t)
}

// dateNew: (Int, Int, Int, offset?: Int) -> Date
// offset defaults to local offset
func builtinDateNew(e *Evaluator, args ...Object) Object {
	if len(args) < 3 || len(args) > 4 {
		return newError("dateNew expects 3 or 4 arguments, got %d", len(args))
	}

	year, ok := args[0].(*Integer)
	if !ok {
		return newError("dateNew expects integer year, got %s", args[0].Type())
	}

	month, ok := args[1].(*Integer)
	if !ok {
		return newError("dateNew expects integer month, got %s", args[1].Type())
	}

	day, ok := args[2].(*Integer)
	if !ok {
		return newError("dateNew expects integer day, got %s", args[2].Type())
	}

	// Default to local offset
	offsetMinutes := getLocalOffset()
	if len(args) == 4 {
		offset, ok := args[3].(*Integer)
		if !ok {
			return newError("dateNew expects integer offset, got %s", args[3].Type())
		}
		offsetMinutes = offset.Value
	}

	loc := time.FixedZone("", int(offsetMinutes)*60)
	t := time.Date(int(year.Value), time.Month(month.Value), int(day.Value), 0, 0, 0, 0, loc)
	return makeDateWithOffset(t, offsetMinutes)
}

// dateNewTime: (Int, Int, Int, Int, Int, Int, offset?: Int) -> Date
// offset defaults to local offset
func builtinDateNewTime(e *Evaluator, args ...Object) Object {
	if len(args) < 6 || len(args) > 7 {
		return newError("dateNewTime expects 6 or 7 arguments, got %d", len(args))
	}

	year, ok := args[0].(*Integer)
	if !ok {
		return newError("dateNewTime expects integer year, got %s", args[0].Type())
	}

	month, ok := args[1].(*Integer)
	if !ok {
		return newError("dateNewTime expects integer month, got %s", args[1].Type())
	}

	day, ok := args[2].(*Integer)
	if !ok {
		return newError("dateNewTime expects integer day, got %s", args[2].Type())
	}

	hour, ok := args[3].(*Integer)
	if !ok {
		return newError("dateNewTime expects integer hour, got %s", args[3].Type())
	}

	minute, ok := args[4].(*Integer)
	if !ok {
		return newError("dateNewTime expects integer minute, got %s", args[4].Type())
	}

	second, ok := args[5].(*Integer)
	if !ok {
		return newError("dateNewTime expects integer second, got %s", args[5].Type())
	}

	// Default to local offset
	offsetMinutes := getLocalOffset()
	if len(args) == 7 {
		offset, ok := args[6].(*Integer)
		if !ok {
			return newError("dateNewTime expects integer offset, got %s", args[6].Type())
		}
		offsetMinutes = offset.Value
	}

	loc := time.FixedZone("", int(offsetMinutes)*60)
	t := time.Date(int(year.Value), time.Month(month.Value), int(day.Value),
		int(hour.Value), int(minute.Value), int(second.Value), 0, loc)
	return makeDateWithOffset(t, offsetMinutes)
}

// dateToTimestamp: (Date) -> Int
// Correctly converts date with any offset to Unix timestamp
func builtinDateToTimestamp(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("dateToTimestamp expects 1 argument, got %d", len(args))
	}

	date, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("dateToTimestamp expects a Date record, got %s", args[0].Type())
	}

	t, ok := dateToTime(date)
	if !ok {
		return newError("dateToTimestamp: invalid Date record")
	}

	return &Integer{Value: t.Unix()}
}

// dateToUtc: (Date) -> Date (converts to offset=0)
func builtinDateToUtc(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("dateToUtc expects 1 argument, got %d", len(args))
	}

	date, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("dateToUtc expects a Date record, got %s", args[0].Type())
	}

	t, ok := dateToTime(date)
	if !ok {
		return newError("dateToUtc: invalid Date record")
	}

	return makeDateWithOffset(t.UTC(), 0)
}

// dateToLocal: (Date) -> Date (converts to local offset)
func builtinDateToLocal(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("dateToLocal expects 1 argument, got %d", len(args))
	}

	date, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("dateToLocal expects a Date record, got %s", args[0].Type())
	}

	t, ok := dateToTime(date)
	if !ok {
		return newError("dateToLocal: invalid Date record")
	}

	return makeDate(t.Local())
}

// dateOffset: (Date) -> Int (get offset in minutes)
func builtinDateOffset(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("dateOffset expects 1 argument, got %d", len(args))
	}

	date, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("dateOffset expects a Date record, got %s", args[0].Type())
	}

	return &Integer{Value: getDateOffset(date)}
}

// dateWithOffset: (Date, Int) -> Date (change offset, adjusting time)
func builtinDateWithOffset(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("dateWithOffset expects 2 arguments, got %d", len(args))
	}

	date, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("dateWithOffset expects a Date record, got %s", args[0].Type())
	}

	newOffset, ok := args[1].(*Integer)
	if !ok {
		return newError("dateWithOffset expects integer offset, got %s", args[1].Type())
	}

	t, ok := dateToTime(date)
	if !ok {
		return newError("dateWithOffset: invalid Date record")
	}

	// Convert to new timezone
	newLoc := time.FixedZone("", int(newOffset.Value)*60)
	return makeDateWithOffset(t.In(newLoc), newOffset.Value)
}

// Convert our format string to Go's format
// YYYY -> 2006, YY -> 06, MM -> 01, DD -> 02, HH -> 15, mm -> 04, ss -> 05
func convertDateFormat(format string) string {
	result := format
	// Replace in order to avoid conflicts (YYYY before YY)
	replacements := []struct{ from, to string }{
		{"YYYY", "2006"},
		{"YY", "06"},
		{"MM", "01"},
		{"DD", "02"},
		{"HH", "15"},
		{"mm", "04"},
		{"ss", "05"},
	}
	for _, r := range replacements {
		result = replaceAll(result, r.from, r.to)
	}
	return result
}

func replaceAll(s, old, new string) string {
	result := ""
	for i := 0; i < len(s); {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			result += new
			i += len(old)
		} else {
			result += string(s[i])
			i++
		}
	}
	return result
}

// dateFormat: (Date, String) -> String
func builtinDateFormat(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("dateFormat expects 2 arguments, got %d", len(args))
	}

	date, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("dateFormat expects a Date record, got %s", args[0].Type())
	}

	formatList, ok := args[1].(*List)
	if !ok {
		return newError("dateFormat expects a string format, got %s", args[1].Type())
	}

	t, ok := dateToTime(date)
	if !ok {
		return newError("dateFormat: invalid Date record")
	}

	format := listToString(formatList)
	goFormat := convertDateFormat(format)
	return stringToList(t.Format(goFormat))
}

// dateParse: (String, String) -> Option<Date>
func builtinDateParse(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("dateParse expects 2 arguments, got %d", len(args))
	}

	strList, ok := args[0].(*List)
	if !ok {
		return newError("dateParse expects a string, got %s", args[0].Type())
	}

	formatList, ok := args[1].(*List)
	if !ok {
		return newError("dateParse expects a string format, got %s", args[1].Type())
	}

	str := listToString(strList)
	format := listToString(formatList)
	goFormat := convertDateFormat(format)

	t, err := time.ParseInLocation(goFormat, str, time.Local)
	if err != nil {
		return makeNone()
	}

	return makeSome(makeDate(t))
}

// Component getters
func builtinDateYear(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("dateYear expects 1 argument, got %d", len(args))
	}
	date, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("dateYear expects a Date record, got %s", args[0].Type())
	}
	if yearObj := date.Get("year"); yearObj != nil {
		return yearObj
	}
	return newError("dateYear: invalid Date record")
}

func builtinDateMonth(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("dateMonth expects 1 argument, got %d", len(args))
	}
	date, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("dateMonth expects a Date record, got %s", args[0].Type())
	}
	if monthObj := date.Get("month"); monthObj != nil {
		return monthObj
	}
	return newError("dateMonth: invalid Date record")
}

func builtinDateDay(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("dateDay expects 1 argument, got %d", len(args))
	}
	date, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("dateDay expects a Date record, got %s", args[0].Type())
	}
	if dayObj := date.Get("day"); dayObj != nil {
		return dayObj
	}
	return newError("dateDay: invalid Date record")
}

func builtinDateWeekday(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("dateWeekday expects 1 argument, got %d", len(args))
	}
	date, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("dateWeekday expects a Date record, got %s", args[0].Type())
	}
	t, ok := dateToTime(date)
	if !ok {
		return newError("dateWeekday: invalid Date record")
	}
	return &Integer{Value: int64(t.Weekday())} // 0 = Sunday
}

func builtinDateHour(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("dateHour expects 1 argument, got %d", len(args))
	}
	date, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("dateHour expects a Date record, got %s", args[0].Type())
	}
	if hourObj := date.Get("hour"); hourObj != nil {
		return hourObj
	}
	return newError("dateHour: invalid Date record")
}

func builtinDateMinute(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("dateMinute expects 1 argument, got %d", len(args))
	}
	date, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("dateMinute expects a Date record, got %s", args[0].Type())
	}
	if minuteObj := date.Get("minute"); minuteObj != nil {
		return minuteObj
	}
	return newError("dateMinute: invalid Date record")
}

func builtinDateSecond(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("dateSecond expects 1 argument, got %d", len(args))
	}
	date, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("dateSecond expects a Date record, got %s", args[0].Type())
	}
	if secondObj := date.Get("second"); secondObj != nil {
		return secondObj
	}
	return newError("dateSecond: invalid Date record")
}

// Arithmetic functions - preserve offset
func builtinDateAddDays(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("dateAddDays expects 2 arguments, got %d", len(args))
	}
	date, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("dateAddDays expects a Date record, got %s", args[0].Type())
	}
	n, ok := args[1].(*Integer)
	if !ok {
		return newError("dateAddDays expects an integer, got %s", args[1].Type())
	}
	t, ok := dateToTime(date)
	if !ok {
		return newError("dateAddDays: invalid Date record")
	}
	return makeDateWithOffset(t.AddDate(0, 0, int(n.Value)), getDateOffset(date))
}

func builtinDateAddMonths(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("dateAddMonths expects 2 arguments, got %d", len(args))
	}
	date, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("dateAddMonths expects a Date record, got %s", args[0].Type())
	}
	n, ok := args[1].(*Integer)
	if !ok {
		return newError("dateAddMonths expects an integer, got %s", args[1].Type())
	}
	t, ok := dateToTime(date)
	if !ok {
		return newError("dateAddMonths: invalid Date record")
	}
	return makeDateWithOffset(t.AddDate(0, int(n.Value), 0), getDateOffset(date))
}

func builtinDateAddYears(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("dateAddYears expects 2 arguments, got %d", len(args))
	}
	date, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("dateAddYears expects a Date record, got %s", args[0].Type())
	}
	n, ok := args[1].(*Integer)
	if !ok {
		return newError("dateAddYears expects an integer, got %s", args[1].Type())
	}
	t, ok := dateToTime(date)
	if !ok {
		return newError("dateAddYears: invalid Date record")
	}
	return makeDateWithOffset(t.AddDate(int(n.Value), 0, 0), getDateOffset(date))
}

func builtinDateAddHours(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("dateAddHours expects 2 arguments, got %d", len(args))
	}
	date, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("dateAddHours expects a Date record, got %s", args[0].Type())
	}
	n, ok := args[1].(*Integer)
	if !ok {
		return newError("dateAddHours expects an integer, got %s", args[1].Type())
	}
	t, ok := dateToTime(date)
	if !ok {
		return newError("dateAddHours: invalid Date record")
	}
	return makeDateWithOffset(t.Add(time.Duration(n.Value)*time.Hour), getDateOffset(date))
}

func builtinDateAddMinutes(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("dateAddMinutes expects 2 arguments, got %d", len(args))
	}
	date, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("dateAddMinutes expects a Date record, got %s", args[0].Type())
	}
	n, ok := args[1].(*Integer)
	if !ok {
		return newError("dateAddMinutes expects an integer, got %s", args[1].Type())
	}
	t, ok := dateToTime(date)
	if !ok {
		return newError("dateAddMinutes: invalid Date record")
	}
	return makeDateWithOffset(t.Add(time.Duration(n.Value)*time.Minute), getDateOffset(date))
}

func builtinDateAddSeconds(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("dateAddSeconds expects 2 arguments, got %d", len(args))
	}
	date, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("dateAddSeconds expects a Date record, got %s", args[0].Type())
	}
	n, ok := args[1].(*Integer)
	if !ok {
		return newError("dateAddSeconds expects an integer, got %s", args[1].Type())
	}
	t, ok := dateToTime(date)
	if !ok {
		return newError("dateAddSeconds: invalid Date record")
	}
	return makeDateWithOffset(t.Add(time.Duration(n.Value)*time.Second), getDateOffset(date))
}

// Difference functions
func builtinDateDiffDays(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("dateDiffDays expects 2 arguments, got %d", len(args))
	}
	date1, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("dateDiffDays expects a Date record, got %s", args[0].Type())
	}
	date2, ok := args[1].(*RecordInstance)
	if !ok {
		return newError("dateDiffDays expects a Date record, got %s", args[1].Type())
	}
	t1, ok := dateToTime(date1)
	if !ok {
		return newError("dateDiffDays: invalid Date record")
	}
	t2, ok := dateToTime(date2)
	if !ok {
		return newError("dateDiffDays: invalid Date record")
	}
	diff := t1.Sub(t2)
	return &Integer{Value: int64(diff.Hours() / 24)}
}

func builtinDateDiffSeconds(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("dateDiffSeconds expects 2 arguments, got %d", len(args))
	}
	date1, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("dateDiffSeconds expects a Date record, got %s", args[0].Type())
	}
	date2, ok := args[1].(*RecordInstance)
	if !ok {
		return newError("dateDiffSeconds expects a Date record, got %s", args[1].Type())
	}
	t1, ok := dateToTime(date1)
	if !ok {
		return newError("dateDiffSeconds: invalid Date record")
	}
	t2, ok := dateToTime(date2)
	if !ok {
		return newError("dateDiffSeconds: invalid Date record")
	}
	diff := t1.Sub(t2)
	return &Integer{Value: int64(diff.Seconds())}
}

// SetDateBuiltinTypes sets type info for date builtins
func SetDateBuiltinTypes(builtins map[string]*Builtin) {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}

	// Date = { year: Int, month: Int, day: Int, hour: Int, minute: Int, second: Int, offset: Int }
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
		Constructor: typesystem.TCon{Name: "Option"},
		Args:        []typesystem.Type{dateType},
	}

	types := map[string]typesystem.Type{
		// Creation
		"dateNow":           typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: dateType},
		"dateNowUtc":        typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: dateType},
		"dateFromTimestamp": typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: dateType},
		// dateNew has optional offset (handled by DefaultCount)
		"dateNew": typesystem.TFunc{
			Params:       []typesystem.Type{typesystem.Int, typesystem.Int, typesystem.Int, typesystem.Int},
			ReturnType:   dateType,
			DefaultCount: 1,
		},
		// dateNewTime has optional offset
		"dateNewTime": typesystem.TFunc{
			Params:       []typesystem.Type{typesystem.Int, typesystem.Int, typesystem.Int, typesystem.Int, typesystem.Int, typesystem.Int, typesystem.Int},
			ReturnType:   dateType,
			DefaultCount: 1,
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
	}

	for name, typ := range types {
		if b, ok := builtins[name]; ok {
			b.TypeInfo = typ
		}
	}
}
