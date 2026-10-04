package cron

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// probeMemory reads /proc/meminfo and returns the fraction of memory used.
// Returns (usedFraction, error) where usedFraction is 0.0-1.0.
func probeMemory() (float64, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	var memTotal, memAvailable uint64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			memTotal = parseMeminfoValue(line)
		} else if strings.HasPrefix(line, "MemAvailable:") {
			memAvailable = parseMeminfoValue(line)
		}
	}

	if memTotal == 0 {
		return 0, nil
	}

	used := memTotal - memAvailable
	return float64(used) / float64(memTotal), nil
}

// parseMeminfoValue extracts the kB value from a /proc/meminfo line.
// Format: "MemTotal:       16384000 kB"
func parseMeminfoValue(line string) uint64 {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	val, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0
	}
	return val * 1024 // convert kB to bytes
}
