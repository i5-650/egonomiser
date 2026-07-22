package sizing

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"

	"e-go-nomiser/internal/types"
)

func defaultParams() Params {
	return Params{
		OversizedPct:      20,
		UndersizedPct:     100,
		TargetUtilization: 0.6,
	}
}

func TestBuild_Undersized(t *testing.T) {
	assessments := []types.PodAssessment{
		{
			Namespace:       "default",
			Name:            "hungry",
			CPUUsageMilli:   150,
			CPURequestMilli: 100,
			HasCPURequest:   true,
		},
	}
	report := Build(assessments, defaultParams())

	if len(report.Undersized) != 1 {
		t.Fatalf("Undersized = %d entries, want 1 (%+v)", len(report.Undersized), report)
	}
	if len(report.Oversized) != 0 || len(report.NoRequests) != 0 {
		t.Fatalf("expected only Undersized populated, got %+v", report)
	}

	entry := report.Undersized[0]
	if entry.Namespace != "default" || entry.Name != "hungry" {
		t.Errorf("entry = %+v, want namespace/name default/hungry", entry)
	}
	if entry.CPUPercent == nil || *entry.CPUPercent != 150 {
		t.Errorf("CPUPercent = %v, want 150", entry.CPUPercent)
	}
	if entry.CPURecommended != "250m" {
		t.Errorf("CPURecommended = %q, want %q", entry.CPURecommended, "250m")
	}
	if entry.MemRequest != "" {
		t.Errorf("MemRequest = %q, want empty (no mem request set)", entry.MemRequest)
	}
}

func TestBuild_Oversized(t *testing.T) {
	assessments := []types.PodAssessment{
		{
			Namespace:       "default",
			Name:            "idle",
			CPUUsageMilli:   10,
			CPURequestMilli: 100,
			HasCPURequest:   true,
		},
	}
	report := Build(assessments, defaultParams())

	if len(report.Oversized) != 1 {
		t.Fatalf("Oversized = %d entries, want 1 (%+v)", len(report.Oversized), report)
	}
	entry := report.Oversized[0]
	if entry.CPUPercent == nil || *entry.CPUPercent != 10 {
		t.Errorf("CPUPercent = %v, want 10", entry.CPUPercent)
	}
	// ceil(10/0.6) = 17, rounded up to the nearest 5m = 20m.
	if entry.CPURecommended != "20m" {
		t.Errorf("CPURecommended = %q, want %q", entry.CPURecommended, "20m")
	}
}

func TestBuild_Healthy(t *testing.T) {
	assessments := []types.PodAssessment{
		{
			Namespace:       "default",
			Name:            "healthy",
			CPUUsageMilli:   50,
			CPURequestMilli: 100,
			HasCPURequest:   true,
		},
	}
	report := Build(assessments, defaultParams())

	if len(report.Undersized) != 0 || len(report.Oversized) != 0 || len(report.NoRequests) != 0 {
		t.Errorf("expected no buckets populated for a healthy pod, got %+v", report)
	}
}

func TestBuild_NoRequests(t *testing.T) {
	assessments := []types.PodAssessment{
		{Namespace: "default", Name: "bare"},
	}
	report := Build(assessments, defaultParams())

	if len(report.NoRequests) != 1 || report.NoRequests[0] != "default/bare" {
		t.Fatalf("NoRequests = %+v, want [\"default/bare\"]", report.NoRequests)
	}
	if len(report.Undersized) != 0 || len(report.Oversized) != 0 {
		t.Errorf("expected only NoRequests populated, got %+v", report)
	}
}

func TestBuild_MemoryOnly(t *testing.T) {
	assessments := []types.PodAssessment{
		{
			Namespace:       "default",
			Name:            "mem-hungry",
			MemUsageBytes:   150,
			MemRequestBytes: 100,
			HasMemRequest:   true,
		},
	}
	report := Build(assessments, defaultParams())

	if len(report.Undersized) != 1 {
		t.Fatalf("Undersized = %d entries, want 1", len(report.Undersized))
	}
	entry := report.Undersized[0]
	if entry.CPURequest != "" {
		t.Errorf("CPURequest = %q, want empty (no cpu request set)", entry.CPURequest)
	}
	if entry.MemPercent == nil || *entry.MemPercent != 150 {
		t.Errorf("MemPercent = %v, want 150", entry.MemPercent)
	}
}

func TestBuild_ThresholdsAndTargetPropagate(t *testing.T) {
	params := Params{OversizedPct: 30, UndersizedPct: 90, TargetUtilization: 0.5}
	report := Build(nil, params)

	if report.OversizedThresholdPct != 30 || report.UndersizedThresholdPct != 90 || report.TargetUtilization != 0.5 {
		t.Errorf("report thresholds = %+v, want OversizedThresholdPct=30 UndersizedThresholdPct=90 TargetUtilization=0.5", report)
	}
}

func TestRecommendCPU(t *testing.T) {
	cases := []struct {
		name              string
		usage             string
		targetUtilization float64
		wantMilli         int64
	}{
		{"exact multiple of step", "50m", 0.5, 100},
		{"rounds up to step", "10m", 0.6, 20},
		{"floors at step for zero usage", "0", 0.6, 5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := recommendCPU(resource.MustParse(c.usage), c.targetUtilization)
			if got.MilliValue() != c.wantMilli {
				t.Errorf("recommendCPU(%s, %v) = %dm, want %dm", c.usage, c.targetUtilization, got.MilliValue(), c.wantMilli)
			}
		})
	}
}

func TestRecommendMemory(t *testing.T) {
	cases := []struct {
		name              string
		usage             string
		targetUtilization float64
		wantBytes         int64
	}{
		{"exact multiple of step", "4Mi", 0.5, 8 * 1024 * 1024},
		{"floors at step for zero usage", "0", 0.6, 8 * 1024 * 1024},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := recommendMemory(resource.MustParse(c.usage), c.targetUtilization)
			if got.Value() != c.wantBytes {
				t.Errorf("recommendMemory(%s, %v) = %d bytes, want %d", c.usage, c.targetUtilization, got.Value(), c.wantBytes)
			}
		})
	}
}

func TestRoundUpToStep(t *testing.T) {
	cases := []struct {
		v, step, want int64
	}{
		{12, 5, 15},
		{15, 5, 15},
		{0, 5, 0},
		{1, 0, 1}, // step <= 0 is a no-op
	}
	for _, c := range cases {
		if got := roundUpToStep(c.v, c.step); got != c.want {
			t.Errorf("roundUpToStep(%d, %d) = %d, want %d", c.v, c.step, got, c.want)
		}
	}
}
