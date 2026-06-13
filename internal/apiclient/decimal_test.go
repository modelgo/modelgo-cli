package apiclient

import (
	"encoding/json"
	"testing"
)

func TestDecimalUnmarshal(t *testing.T) {
	type wrap struct {
		Amount Decimal `json:"amount"`
	}
	cases := []struct {
		name, body, wantStr string
		wantFloat           float64
	}{
		{"quoted decimal", `{"amount":"99999995.5796595648"}`, "99999995.5796595648", 99999995.5796595648},
		{"quoted zero", `{"amount":"0"}`, "0", 0},
		{"bare number", `{"amount":12.5}`, "12.5", 12.5},
		{"bare int", `{"amount":7}`, "7", 7},
		{"null", `{"amount":null}`, "0", 0},
		{"missing", `{}`, "0", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var w wrap
			if err := json.Unmarshal([]byte(tc.body), &w); err != nil {
				t.Fatalf("unmarshal %s: %v", tc.body, err)
			}
			if got := w.Amount.String(); got != tc.wantStr {
				t.Errorf("String() = %q, want %q", got, tc.wantStr)
			}
			if got := w.Amount.Float(); got != tc.wantFloat {
				t.Errorf("Float() = %v, want %v", got, tc.wantFloat)
			}
		})
	}
}

// Round-trip: a decoded Decimal re-marshals as a JSON string so `--json` output
// stays faithful to the upstream encoding.
func TestDecimalMarshalIsString(t *testing.T) {
	var d Decimal
	if err := json.Unmarshal([]byte(`"12.34"`), &d); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `"12.34"` {
		t.Errorf("Marshal = %s, want \"12.34\"", b)
	}
}
