package exchange

import "testing"

func TestParseLine(t *testing.T) {
	tests := []struct {
		line        string
		version     string
		cacheStatus string
	}{
		{"1795000000.000\t0.125\t200\tEXPIRED\t/api/v1/exchange-rates", "v1", "EXPIRED"},
		{"1795000001.000\t0.002\t200\tHIT\t/api/v2/exchange-rates/20261118/20261118120001.json", "v2", "HIT"},
		{"1795000002.000\t0.020\t200\tMISS\t/api/v2/exchange-rates/20261118/20261118120002.json", "v2", "MISS"},
	}
	for _, test := range tests {
		record, err := ParseLine(test.line)
		if err != nil {
			t.Fatal(err)
		}
		if record.Version != test.version || record.CacheStatus != test.cacheStatus {
			t.Errorf("ParseLine(%q) = %+v", test.line, record)
		}
	}
}

func TestParseLineRejectsOtherRoutes(t *testing.T) {
	if _, err := ParseLine("1795000000.000\t0.001\t200\t-\t/api/v1/products"); err == nil {
		t.Fatal("expected unsupported route error")
	}
}
