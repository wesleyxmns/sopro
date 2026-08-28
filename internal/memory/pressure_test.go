package memory

import (
	"strings"
	"testing"
)

func TestParsePSI(t *testing.T) {
	input := "some avg10=1.25 avg60=0.50 avg300=0.10 total=12345\n" +
		"full avg10=0.20 avg60=0.10 avg300=0.01 total=456\n"
	pressure, err := ParsePSI(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if !pressure.Supported || pressure.Some.Avg10 != 1.25 || pressure.Full.TotalMicros != 456 {
		t.Fatalf("unexpected pressure: %+v", pressure)
	}
}

func TestParsePSIRequiresSomeRecord(t *testing.T) {
	_, err := ParsePSI(strings.NewReader("full avg10=0.2 avg60=0.1 avg300=0 total=1\n"))
	if err == nil {
		t.Fatal("expected missing some record to fail")
	}
}
