// Package sizing turns per-pod usage/request assessments into a grouped
// report of undersized/oversized/no-request pods, each with a recommended
// request that would bring usage to a target utilization.
package sizing

import (
	"math"

	"k8s.io/apimachinery/pkg/api/resource"

	"e-go-nomiser/internal/format"
	"e-go-nomiser/internal/types"
)

// Params bundles the thresholds used to classify pods and size
// recommendations.
type Params struct {
	// OversizedPct: flag a pod as oversized if usage stays below this % of
	// its request (for every resource that has a request set).
	OversizedPct int64
	// UndersizedPct: flag a pod as undersized if usage exceeds this % of
	// its request for any resource.
	UndersizedPct int64
	// TargetUtilization: recommended requests aim for usage to sit at this
	// fraction of the request (e.g. 0.6 = 60%).
	TargetUtilization float64
}

// Build groups pods into undersized/oversized/no-request buckets based on
// their used/requested percentages, and for each flagged pod computes a
// recommended request that would bring usage to roughly
// params.TargetUtilization of the request.
//
//   - undersized: any resource's usage exceeded params.UndersizedPct of its
//     request (default 100%) — the pod is relying on borrowed headroom from
//     other pods rather than its own guaranteed share; recommendation
//     raises it.
//   - oversized: every resource that has a request stayed below
//     params.OversizedPct (default 20%) — the request is likely bigger than
//     needed, reserving capacity nobody uses; recommendation lowers it.
//   - pods with no requests set at all aren't sizeable and are reported
//     separately, since there's no baseline to judge them against.
func Build(assessments []types.PodAssessment, params Params) types.SizingReport {
	report := types.SizingReport{
		OversizedThresholdPct:  params.OversizedPct,
		UndersizedThresholdPct: params.UndersizedPct,
		TargetUtilization:      params.TargetUtilization,
	}

	for _, a := range assessments {
		if !a.HasCPURequest && !a.HasMemRequest {
			report.NoRequests = append(report.NoRequests, a.Namespace+"/"+a.Name)
			continue
		}

		var cpuPct, memPct *int64
		if a.HasCPURequest {
			v := format.PercentOfMilli(a.CPUUsageMilli, a.CPURequestMilli)
			cpuPct = &v
		}
		if a.HasMemRequest {
			v := format.PercentOfMilli(a.MemUsageBytes, a.MemRequestBytes)
			memPct = &v
		}

		isUndersized := (cpuPct != nil && *cpuPct > params.UndersizedPct) || (memPct != nil && *memPct > params.UndersizedPct)
		isOversized := !isUndersized &&
			(cpuPct == nil || *cpuPct < params.OversizedPct) &&
			(memPct == nil || *memPct < params.OversizedPct)

		if !isUndersized && !isOversized {
			continue // sits in a healthy range — nothing to recommend
		}

		entry := types.SizingEntry{Namespace: a.Namespace, Name: a.Name}
		if a.HasCPURequest {
			usage := *resource.NewMilliQuantity(a.CPUUsageMilli, resource.DecimalSI)
			request := *resource.NewMilliQuantity(a.CPURequestMilli, resource.DecimalSI)
			rec := recommendCPU(usage, params.TargetUtilization)
			entry.CPUUsage = format.CPU(usage)
			entry.CPURequest = format.CPU(request)
			entry.CPUPercent = cpuPct
			entry.CPURecommended = format.CPU(rec)
		}
		if a.HasMemRequest {
			usage := *resource.NewQuantity(a.MemUsageBytes, resource.BinarySI)
			request := *resource.NewQuantity(a.MemRequestBytes, resource.BinarySI)
			rec := recommendMemory(usage, params.TargetUtilization)
			entry.MemUsage = format.Memory(usage)
			entry.MemRequest = format.Memory(request)
			entry.MemPercent = memPct
			entry.MemRecommended = format.Memory(rec)
		}

		if isUndersized {
			report.Undersized = append(report.Undersized, entry)
		} else {
			report.Oversized = append(report.Oversized, entry)
		}
	}

	return report
}

// recommendCPU suggests a millicore request such that usage would sit at
// roughly targetUtilization of it, rounded up to the nearest 5m and floored
// at 5m (a request of 0 isn't schedulable/meaningful).
func recommendCPU(usage resource.Quantity, targetUtilization float64) resource.Quantity {
	const step = 5 // millicores
	raw := int64(math.Ceil(float64(usage.MilliValue()) / targetUtilization))
	raw = roundUpToStep(raw, step)
	if raw < step {
		raw = step
	}
	return *resource.NewMilliQuantity(raw, resource.DecimalSI)
}

// recommendMemory suggests a byte request such that usage would sit at
// roughly targetUtilization of it, rounded up to the nearest 8Mi and floored
// at 8Mi (a request of 0 isn't schedulable/meaningful).
func recommendMemory(usage resource.Quantity, targetUtilization float64) resource.Quantity {
	const step = 8 * 1024 * 1024 // 8Mi, in bytes
	raw := int64(math.Ceil(float64(usage.Value()) / targetUtilization))
	raw = roundUpToStep(raw, step)
	if raw < step {
		raw = step
	}
	return *resource.NewQuantity(raw, resource.BinarySI)
}

// roundUpToStep rounds v up to the nearest multiple of step, so recommended
// values look like requests a human would actually write (e.g. "245m", not
// "243.7861m"), rather than a raw division result.
func roundUpToStep(v, step int64) int64 {
	if step <= 0 {
		return v
	}
	return ((v + step - 1) / step) * step
}
