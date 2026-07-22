package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"e-go-nomiser/internal/types"
)

func sampleReport() types.Report {
	cpuPct := int64(45)
	memPct := int64(70)
	sizingPct := int64(150)

	return types.Report{
		Timestamp: time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC),
		Nodes: []types.NodeUsage{
			{
				Name:                "node-a",
				CPUUsedMilli:        45,
				CPUAllocatableMilli: 100,
				CPUPercent:          &cpuPct,
				MemUsedBytes:        700,
				MemAllocatableBytes: 1000,
				MemPercent:          &memPct,
			},
		},
		Pods: []types.PodUsage{
			{
				Namespace:       "default",
				Name:            "web",
				CPUUsedMilli:    45,
				CPURequestMilli: 100,
				HasCPURequest:   true,
				CPUPercent:      &cpuPct,
			},
		},
		Sizing: types.SizingReport{
			Undersized: []types.SizingEntry{
				{
					Namespace:      "default",
					Name:           "hungry",
					CPUUsage:       "150m",
					CPURequest:     "100m",
					CPUPercent:     &sizingPct,
					CPURecommended: "250m",
				},
			},
			NoRequests:             []string{"default/bare"},
			OversizedThresholdPct:  20,
			UndersizedThresholdPct: 100,
			TargetUtilization:      0.6,
		},
	}
}

func TestText(t *testing.T) {
	var buf bytes.Buffer
	if err := Text(&buf, sampleReport()); err != nil {
		t.Fatalf("Text() error = %v", err)
	}
	out := buf.String()

	wantSubstrings := []string{
		"=== Node usage vs allocatable",
		"node-a",
		"=== Pod usage vs requested",
		"default",
		"web",
		"=== Sizing report ===",
		"Undersized (using >100% of a request):",
		"default/hungry",
		"150m used / 100m requested (150%) -> recommend ~250m",
		"Oversized (using <20% of every request it has):",
		"(none)",
		"No requests set (can't be sized against anything):",
		"default/bare",
		"Recommendations target ~60% utilization",
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(out, want) {
			t.Errorf("Text() output missing %q\n--- full output ---\n%s", want, out)
		}
	}
}

func TestText_EmptyReport(t *testing.T) {
	var buf bytes.Buffer
	if err := Text(&buf, types.Report{}); err != nil {
		t.Fatalf("Text() error = %v", err)
	}
	out := buf.String()

	// With nothing to report, both buckets should show "(none)" rather than
	// panicking or leaving the section blank.
	if strings.Count(out, "(none)") != 2 {
		t.Errorf("Text() output = %q, want exactly 2 occurrences of \"(none)\"", out)
	}
	if strings.Contains(out, "No requests set") {
		t.Errorf("Text() output should omit the no-requests section when there are none:\n%s", out)
	}
}
