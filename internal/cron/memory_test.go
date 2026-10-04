package cron

import (
	"testing"
)

func TestProbeMemory(t *testing.T) {
	frac, err := probeMemory()
	if err != nil {
		t.Fatalf("probeMemory error: %v", err)
	}
	if frac < 0 || frac > 1 {
		t.Errorf("probeMemory = %f, want 0.0-1.0", frac)
	}
	t.Logf("memory used: %.2f%%", frac*100)
}

func TestParseMeminfoValue(t *testing.T) {
	tests := []struct {
		line string
		want uint64
	}{
		{"MemTotal:       16384000 kB", 16384000 * 1024},
		{"MemAvailable:    8192000 kB", 8192000 * 1024},
		{"invalid", 0},
		{"MemTotal:", 0},
	}
	
	for _, tt := range tests {
		got := parseMeminfoValue(tt.line)
		if got != tt.want {
			t.Errorf("parseMeminfoValue(%q) = %d, want %d", tt.line, got, tt.want)
		}
	}
}
