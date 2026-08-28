package memory

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type PressureWindow struct {
	Avg10       float64
	Avg60       float64
	Avg300      float64
	TotalMicros uint64
}

type Pressure struct {
	Supported bool
	Some      PressureWindow
	Full      PressureWindow
}

func ParsePSI(reader io.Reader) (Pressure, error) {
	var pressure Pressure
	foundSome := false
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		window, err := parsePressureWindow(fields[1:])
		if err != nil {
			return Pressure{}, fmt.Errorf("parse PSI %s: %w", fields[0], err)
		}
		switch fields[0] {
		case "some":
			pressure.Some = window
			foundSome = true
		case "full":
			pressure.Full = window
		}
	}
	if err := scanner.Err(); err != nil {
		return Pressure{}, err
	}
	if !foundSome {
		return Pressure{}, fmt.Errorf("PSI memory does not contain a some record")
	}
	pressure.Supported = true
	return pressure, nil
}

func parsePressureWindow(fields []string) (PressureWindow, error) {
	values := make(map[string]string, len(fields))
	for _, field := range fields {
		key, value, ok := strings.Cut(field, "=")
		if ok {
			values[key] = value
		}
	}
	parseFloat := func(key string) (float64, error) {
		value, ok := values[key]
		if !ok {
			return 0, fmt.Errorf("missing %s", key)
		}
		return strconv.ParseFloat(value, 64)
	}
	avg10, err := parseFloat("avg10")
	if err != nil {
		return PressureWindow{}, err
	}
	avg60, err := parseFloat("avg60")
	if err != nil {
		return PressureWindow{}, err
	}
	avg300, err := parseFloat("avg300")
	if err != nil {
		return PressureWindow{}, err
	}
	total, err := strconv.ParseUint(values["total"], 10, 64)
	if err != nil {
		return PressureWindow{}, fmt.Errorf("invalid total: %w", err)
	}
	return PressureWindow{Avg10: avg10, Avg60: avg60, Avg300: avg300, TotalMicros: total}, nil
}
