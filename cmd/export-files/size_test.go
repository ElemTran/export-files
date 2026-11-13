package main

import "testing"

func TestParseSize(t *testing.T) {
	tests := []struct {
		in   string
		want int64
	}{
		{"1024", 1024},
		{"10K", 10 << 10},
		{"10k", 10 << 10},
		{"10KB", 10 << 10},
		{"5M", 5 << 20},
		{"1G", 1 << 30},
		{"2B", 2},
		{" 7 ", 7},
	}
	for _, tt := range tests {
		got, err := parseSize(tt.in)
		if err != nil {
			t.Errorf("parseSize(%q) 报错: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseSize(%q) = %d, 期望 %d", tt.in, got, tt.want)
		}
	}
}

func TestParseSizeRejectsInvalid(t *testing.T) {
	for _, bad := range []string{"", "K", "-5", "1.5M", "abc"} {
		if _, err := parseSize(bad); err == nil {
			t.Errorf("parseSize(%q) 应报错", bad)
		}
	}
}
