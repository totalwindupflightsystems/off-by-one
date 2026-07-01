package cron

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

// readLoadavg reads /proc/loadavg. The file is in the standard
// Linux format: "0.42 0.31 0.28 1/123 4567\n". We return the raw
// bytes so parseLoadavg can run in isolation in tests.
func readLoadavg() ([]byte, error) {
	return os.ReadFile("/proc/loadavg")
}

// ErrMalformedLoadavg signals the loadavg file is missing
// expected fields. Parsing falls back to zero with this error so
// the loop's IdleProbe path can log it.
var ErrMalformedLoadavg = errors.New("cron: malformed /proc/loadavg")

// parseLoadavg extracts the first whitespace-delimited field and
// parses it as a float. The first field is the 1-minute load
// average; the next two are 5- and 15-minute averages (unused
// here — see probeLoadavg's docs).
func parseLoadavg(data []byte) (float64, error) {
	s := strings.TrimSpace(string(data))
	if s == "" {
		return 0, ErrMalformedLoadavg
	}
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return 0, ErrMalformedLoadavg
	}
	v, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, ErrMalformedLoadavg
	}
	return v, nil
}
