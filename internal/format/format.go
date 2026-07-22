// Package format renders raw resource quantities as human-readable strings
// and computes usage percentages, shared by the collector, sizing, and
// output packages.
package format

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"
)

// PercentOf returns usage/total * 100 as an integer, guarding against
// div-by-zero. Uses the raw quantities (not the display strings), so it
// stays precise regardless of what scale each value happened to be
// reported in.
func PercentOf(usage, total resource.Quantity) int64 {
	totalMilli := total.MilliValue()
	if totalMilli == 0 {
		return 0
	}
	return usage.MilliValue() * 100 / totalMilli
}

// PercentOfMilli is PercentOf for values already expressed as milli-units
// (millicores or milli-bytes), avoiding a resource.Quantity round-trip.
func PercentOfMilli(usageMilli, totalMilli int64) int64 {
	if totalMilli == 0 {
		return 0
	}
	return usageMilli * 100 / totalMilli
}

// CPU renders CPU as whole millicores, e.g. "1443368n" -> "2m" (rounded up)
// and "100m" -> "100m". This is the same unit kubectl itself displays CPU
// in, and it collapses the nanocore/millicore mismatch that metrics-server
// vs. requests/allocatable otherwise report in.
func CPU(q resource.Quantity) string {
	return fmt.Sprintf("%dm", q.MilliValue())
}

// CPUMilli is CPU for a value already expressed in millicores.
func CPUMilli(milli int64) string {
	return fmt.Sprintf("%dm", milli)
}

// Memory renders a byte quantity auto-scaled to the most readable binary
// unit (Ki/Mi/Gi), e.g. "134217728" -> "128Mi".
func Memory(q resource.Quantity) string {
	return MemoryBytes(q.Value())
}

// MemoryBytes is Memory for a value already expressed as a byte count.
func MemoryBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1fGi", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.0fMi", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.0fKi", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%dB", b)
	}
}

// RatioString formats "used/allocated (pct%)" using the given formatter for
// human-readable units, e.g. "45m/100m (45%)" or "180Mi/256Mi (70%)".
// If allocated is unset (no request/allocatable found, or it's zero — e.g. a
// pod with no CPU request), it prints "used/-" since a percentage would be
// meaningless (undefined or would imply infinite headroom).
func RatioString(used, allocated resource.Quantity, format func(resource.Quantity) string) string {
	if allocated.IsZero() {
		return fmt.Sprintf("%s/-", format(used))
	}
	return fmt.Sprintf("%s/%s (%d%%)", format(used), format(allocated), PercentOf(used, allocated))
}

// RatioStringRaw is RatioString for values already reduced to plain int64
// units (whatever unit formatFn expects) plus a precomputed percentage. A
// nil pct means "no baseline to compare against" (e.g. no request set), so
// it prints "used/-" instead of a percentage.
func RatioStringRaw(used, allocated int64, pct *int64, formatFn func(int64) string) string {
	if pct == nil {
		return fmt.Sprintf("%s/-", formatFn(used))
	}
	return fmt.Sprintf("%s/%s (%d%%)", formatFn(used), formatFn(allocated), *pct)
}
