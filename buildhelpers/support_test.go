package buildhelpers

import (
	"testing"
	"time"
)

func TestSupportDeadline(t *testing.T) {
	p := Package{Target: "alpine/3.24/php/8.4", SupportedUntil: "2026-11-20"}
	for _, tc := range []struct {
		date string
		ok   bool
	}{{"2026-11-19", true}, {"2026-11-20", true}, {"2026-11-21", false}} {
		now, _ := time.Parse("2006-01-02", tc.date)
		if err := CheckSupport(p, now); (err == nil) != tc.ok {
			t.Fatalf("%s: %v", tc.date, err)
		}
	}
	p.SupportedUntil = ""
	if CheckSupport(p, time.Now()) == nil {
		t.Fatal("accepted missing deadline")
	}
}
