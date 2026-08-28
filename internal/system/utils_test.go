package system

import "testing"

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    uint64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024 * 1024, "1.00 MB"},
		{1024 * 1024 * 1024 * 2, "2.00 GB"},
		{1024 * 1024 * 1024 * 1024 * 1024, "1.00 PB"},
	}

	for _, tt := range tests {
		result := FormatBytes(tt.bytes)
		if result != tt.expected {
			t.Errorf("FormatBytes(%d) = %s; want %s", tt.bytes, result, tt.expected)
		}
	}
}
