package format

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
)

func TestPercentOf(t *testing.T) {
	cases := []struct {
		name         string
		usage, total string
		want         int64
	}{
		{"half", "50m", "100m", 50},
		{"zero total", "50m", "0", 0},
		{"over 100", "150m", "100m", 150},
		{"memory", "512Mi", "1Gi", 50},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			usage := resource.MustParse(c.usage)
			total := resource.MustParse(c.total)
			if got := PercentOf(usage, total); got != c.want {
				t.Errorf("PercentOf(%s, %s) = %d, want %d", c.usage, c.total, got, c.want)
			}
		})
	}
}

func TestPercentOfMilli(t *testing.T) {
	cases := []struct {
		name         string
		usage, total int64
		want         int64
	}{
		{"half", 50, 100, 50},
		{"zero total", 50, 0, 0},
		{"over 100", 150, 100, 150},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := PercentOfMilli(c.usage, c.total); got != c.want {
				t.Errorf("PercentOfMilli(%d, %d) = %d, want %d", c.usage, c.total, got, c.want)
			}
		})
	}
}

func TestCPU(t *testing.T) {
	cases := []struct {
		name string
		qty  string
		want string
	}{
		{"millicores", "100m", "100m"},
		{"nanocores rounds up", "1443368n", "2m"},
		{"whole core", "2", "2000m"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CPU(resource.MustParse(c.qty)); got != c.want {
				t.Errorf("CPU(%s) = %q, want %q", c.qty, got, c.want)
			}
		})
	}
}

func TestCPUMilli(t *testing.T) {
	if got := CPUMilli(1500); got != "1500m" {
		t.Errorf("CPUMilli(1500) = %q, want %q", got, "1500m")
	}
}

func TestMemory(t *testing.T) {
	cases := []struct {
		name string
		qty  string
		want string
	}{
		{"bytes", "512", "512B"},
		{"kibibytes", "2Ki", "2Ki"},
		{"mebibytes", "128Mi", "128Mi"},
		{"gibibytes", "2Gi", "2.0Gi"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Memory(resource.MustParse(c.qty)); got != c.want {
				t.Errorf("Memory(%s) = %q, want %q", c.qty, got, c.want)
			}
		})
	}
}

func TestMemoryBytes(t *testing.T) {
	cases := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"bytes", 512, "512B"},
		{"kibibytes", 2 << 10, "2Ki"},
		{"mebibytes", 128 << 20, "128Mi"},
		{"gibibytes", 2 << 30, "2.0Gi"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := MemoryBytes(c.bytes); got != c.want {
				t.Errorf("MemoryBytes(%d) = %q, want %q", c.bytes, got, c.want)
			}
		})
	}
}

func TestRatioString(t *testing.T) {
	t.Run("with allocation", func(t *testing.T) {
		got := RatioString(resource.MustParse("45m"), resource.MustParse("100m"), CPU)
		if want := "45m/100m (45%)"; got != want {
			t.Errorf("RatioString() = %q, want %q", got, want)
		}
	})

	t.Run("zero allocation", func(t *testing.T) {
		got := RatioString(resource.MustParse("45m"), resource.MustParse("0"), CPU)
		if want := "45m/-"; got != want {
			t.Errorf("RatioString() = %q, want %q", got, want)
		}
	})
}

func TestRatioStringRaw(t *testing.T) {
	t.Run("with percent", func(t *testing.T) {
		pct := int64(45)
		got := RatioStringRaw(45, 100, &pct, CPUMilli)
		if want := "45m/100m (45%)"; got != want {
			t.Errorf("RatioStringRaw() = %q, want %q", got, want)
		}
	})

	t.Run("nil percent", func(t *testing.T) {
		got := RatioStringRaw(45, 0, nil, CPUMilli)
		if want := "45m/-"; got != want {
			t.Errorf("RatioStringRaw() = %q, want %q", got, want)
		}
	})
}
