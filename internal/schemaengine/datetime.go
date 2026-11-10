package schemaengine

import "errors"

// LeapPolicy names the leap-second rule applied by the owned date-time and
// time checkers.
//
// structural-leap/v1: second 60 is accepted only when the UTC-equivalent
// instant is 23:59:60 (date-time: on the last day of its month, with full
// day/month/year carry from the offset; time: modulo 24 hours). This is a
// syntactic check. A pass does not establish that a leap second occurred.
// The 0000-9999 range bounds the year as written; carry past that range is
// used only to evaluate the month-end rule.
const LeapPolicy = "structural-leap/v1"

var (
	errDateSyntax     = errors.New("not a full-date (YYYY-MM-DD)")
	errDateRange      = errors.New("month or day out of range")
	errTimeSyntax     = errors.New("not a full-time (HH:MM:SS[.frac] with Z or ±HH:MM offset)")
	errTimeRange      = errors.New("hour, minute, second or offset out of range")
	errLeapSecond     = errors.New("second 60 is only valid at UTC-equivalent 23:59:60")
	errLeapMonthEnd   = errors.New("second 60 is only valid on the last day of a UTC month")
	errDateTimeSyntax = errors.New("not a date-time (full-date T full-time)")
)

// The checkers below apply only to strings. JSON Schema formats do not imply
// a type, so any other instance is valid.

func checkDate(v any) error {
	s, ok := v.(string)
	if !ok {
		return nil
	}
	if len(s) != 10 {
		return errDateSyntax
	}
	y, m, d, ok := parseDate(s)
	if !ok {
		return errDateSyntax
	}
	if !validDate(y, m, d) {
		return errDateRange
	}
	return nil
}

func checkTime(v any) error {
	s, ok := v.(string)
	if !ok {
		return nil
	}
	t, err := parseTime(s)
	if err != nil {
		return err
	}
	if t.sec == 60 {
		utc := t.hour*60 + t.min - t.offset
		if mod(utc, 1440) != 23*60+59 {
			return errLeapSecond
		}
	}
	return nil
}

func checkDateTime(v any) error {
	s, ok := v.(string)
	if !ok {
		return nil
	}
	if len(s) < 11 || (s[10] != 'T' && s[10] != 't') {
		return errDateTimeSyntax
	}
	y, m, d, ok := parseDate(s[:10])
	if !ok {
		return errDateTimeSyntax
	}
	if !validDate(y, m, d) {
		return errDateRange
	}
	t, err := parseTime(s[11:])
	if err != nil {
		return err
	}
	if t.sec == 60 {
		utc := t.hour*60 + t.min - t.offset // range [-1439, 2878]
		if mod(utc, 1440) != 23*60+59 {
			return errLeapSecond
		}
		uy, um, ud := shiftDate(y, m, d, floorDiv(utc, 1440))
		if ud != daysInMonth(uy, um) {
			return errLeapMonthEnd
		}
	}
	return nil
}

type partialTime struct {
	hour, min, sec int
	offset         int // minutes east of UTC
}

// parseTime parses full-time = HH:MM:SS [ "." 1*DIGIT ] ( "Z" / "z" / ("+"/"-") HH:MM ).
// Field ranges are checked here; the leap-second rule is applied by callers.
func parseTime(s string) (partialTime, error) {
	var t partialTime
	if len(s) < 9 || s[2] != ':' || s[5] != ':' {
		return t, errTimeSyntax
	}
	var ok1, ok2, ok3 bool
	t.hour, ok1 = digits2(s[0:2])
	t.min, ok2 = digits2(s[3:5])
	t.sec, ok3 = digits2(s[6:8])
	if !ok1 || !ok2 || !ok3 {
		return t, errTimeSyntax
	}
	i := 8
	if s[i] == '.' {
		j := i + 1
		for j < len(s) && isDigit(s[j]) {
			j++
		}
		if j == i+1 {
			return t, errTimeSyntax
		}
		i = j
	}
	if i >= len(s) {
		return t, errTimeSyntax
	}
	switch s[i] {
	case 'Z', 'z':
		if i+1 != len(s) {
			return t, errTimeSyntax
		}
	case '+', '-':
		rest := s[i+1:]
		if len(rest) != 5 || rest[2] != ':' {
			return t, errTimeSyntax
		}
		oh, okh := digits2(rest[0:2])
		om, okm := digits2(rest[3:5])
		if !okh || !okm {
			return t, errTimeSyntax
		}
		if oh > 23 || om > 59 {
			return t, errTimeRange
		}
		t.offset = oh*60 + om
		if s[i] == '-' {
			t.offset = -t.offset
		}
	default:
		return t, errTimeSyntax
	}
	if t.hour > 23 || t.min > 59 || t.sec > 60 {
		return t, errTimeRange
	}
	return t, nil
}

// parseDate parses exactly YYYY-MM-DD with ASCII digits. It does not check
// calendar validity.
func parseDate(s string) (y, m, d int, ok bool) {
	if len(s) != 10 || s[4] != '-' || s[7] != '-' {
		return 0, 0, 0, false
	}
	hi, ok1 := digits2(s[0:2])
	lo, ok2 := digits2(s[2:4])
	m, ok3 := digits2(s[5:7])
	d, ok4 := digits2(s[8:10])
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return 0, 0, 0, false
	}
	return hi*100 + lo, m, d, true
}

func digits2(s string) (int, bool) {
	if !isDigit(s[0]) || !isDigit(s[1]) {
		return 0, false
	}
	return int(s[0]-'0')*10 + int(s[1]-'0'), true
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isLeapYear(y int) bool { return y%4 == 0 && (y%100 != 0 || y%400 == 0) }

func daysInMonth(y, m int) int {
	switch m {
	case 2:
		if isLeapYear(y) {
			return 29
		}
		return 28
	case 4, 6, 9, 11:
		return 30
	default:
		return 31
	}
}

func validDate(y, m, d int) bool {
	return m >= 1 && m <= 12 && d >= 1 && d <= daysInMonth(y, m)
}

// shiftDate moves a valid date by -1, 0 or +1 days. The year may leave
// 0000-9999; that is only used to evaluate the month-end rule.
func shiftDate(y, m, d, delta int) (int, int, int) {
	switch delta {
	case 1:
		if d < daysInMonth(y, m) {
			return y, m, d + 1
		}
		if m < 12 {
			return y, m + 1, 1
		}
		return y + 1, 1, 1
	case -1:
		if d > 1 {
			return y, m, d - 1
		}
		if m > 1 {
			return y, m - 1, daysInMonth(y, m-1)
		}
		return y - 1, 12, 31
	default:
		return y, m, d
	}
}

func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

func mod(a, b int) int { return a - floorDiv(a, b)*b }
