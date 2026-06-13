package apiclient

import (
	"encoding/json"
	"strconv"
	"strings"
)

// Decimal is a money/decimal amount decoded from JSON that may be either a
// quoted string ("12.34" — the gateway upstreams encode shopspring/decimal as
// JSON strings to preserve precision) or a bare JSON number. It is stored as the
// raw decimal string so no precision is lost on decode.
//
// Use it for any upstream amount field (balance, transaction amount, grant,
// final_amount, spend). Plain float64 fields FAIL to decode the string form
// with "cannot unmarshal string into Go value of type float64".
type Decimal string

// UnmarshalJSON accepts a quoted string, a bare number, or null/empty.
func (d *Decimal) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "" || s == "null" {
		*d = ""
		return nil
	}
	if s[0] == '"' {
		var str string
		if err := json.Unmarshal(b, &str); err != nil {
			return err
		}
		*d = Decimal(strings.TrimSpace(str))
		return nil
	}
	*d = Decimal(s)
	return nil
}

// MarshalJSON emits the amount as a JSON string so `--json` output round-trips
// the upstream encoding faithfully.
func (d Decimal) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// String returns the raw decimal string; an absent/empty amount reads as "0".
func (d Decimal) String() string {
	if d == "" {
		return "0"
	}
	return string(d)
}

// Float returns the best-effort float64 value for display formatting. Precision
// beyond float64 is lost — use String() when exactness matters.
func (d Decimal) Float() float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(string(d)), 64)
	return f
}
