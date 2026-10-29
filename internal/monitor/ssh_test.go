package monitor

import "testing"

func TestProcParsing(t *testing.T) {
	input := "cpu  10 0 20 70 0 0 0 0 0 0\nMemAvailable: 2048 kB\n8 0 sda 5 0 10 0 7 0 14 0 0 0 0\n"
	r, err := parse(input, "sda")
	if err != nil || r.cpu != 100 || r.idle != 70 || r.memory != 2048 || r.reads != 5 || r.writeSectors != 14 {
		t.Fatalf("parse %+v %v", r, err)
	}
	for _, s := range []string{"", "cpu 1\n", "cpu a 0 0 0 0 0 0 0\n"} {
		if _, err := parse(s, ""); err == nil {
			t.Fatal("accepted malformed metrics")
		}
	}
	if _, err := parse(input, "missing"); err == nil {
		t.Fatal("missing disk reported as zero")
	}
}
