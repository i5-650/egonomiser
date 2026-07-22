// Package types holds the plain data structures produced by the collector
// and consumed by the output printers. Keeping them dependency-free (no
// k8s client types, no io.Writer) means they serialize cleanly to JSON and
// can be reused as-is if this tool grows an HTTP API on top later.
package types

import "time"

// NodeUsage is one node's CPU/memory usage vs its allocatable capacity.
type NodeUsage struct {
	Name string `json:"name"`

	CPUUsedMilli        int64  `json:"cpuUsedMilli"`
	CPUAllocatableMilli int64  `json:"cpuAllocatableMilli"`
	CPUPercent          *int64 `json:"cpuPercent,omitempty"`

	MemUsedBytes        int64  `json:"memUsedBytes"`
	MemAllocatableBytes int64  `json:"memAllocatableBytes"`
	MemPercent          *int64 `json:"memPercent,omitempty"`
}

// PodUsage is one pod's CPU/memory usage vs its requested amount.
// HasCPURequest/HasMemRequest being false means that resource had no
// request set, so there's nothing to compute a percentage against.
type PodUsage struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`

	CPUUsedMilli    int64  `json:"cpuUsedMilli"`
	CPURequestMilli int64  `json:"cpuRequestMilli,omitempty"`
	HasCPURequest   bool   `json:"hasCpuRequest"`
	CPUPercent      *int64 `json:"cpuPercent,omitempty"`

	MemUsedBytes    int64  `json:"memUsedBytes"`
	MemRequestBytes int64  `json:"memRequestBytes,omitempty"`
	HasMemRequest   bool   `json:"hasMemRequest"`
	MemPercent      *int64 `json:"memPercent,omitempty"`
}

// PodAssessment is the intermediate, raw form of a pod's usage/request
// quantities that the sizing package consumes to build a SizingReport.
// It's a plain-value copy of what the collector computed per pod, kept
// separate from k8s API types so the sizing package doesn't need to import
// the Kubernetes client libraries.
type PodAssessment struct {
	Namespace string
	Name      string

	CPUUsageMilli   int64
	CPURequestMilli int64
	HasCPURequest   bool

	MemUsageBytes   int64
	MemRequestBytes int64
	HasMemRequest   bool
}

// SizingEntry is one pod's sizing recommendation line, used in both the
// undersized and oversized buckets of a SizingReport.
type SizingEntry struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`

	CPUUsage       string `json:"cpuUsage,omitempty"`
	CPURequest     string `json:"cpuRequest,omitempty"`
	CPUPercent     *int64 `json:"cpuPercent,omitempty"`
	CPURecommended string `json:"cpuRecommended,omitempty"`

	MemUsage       string `json:"memUsage,omitempty"`
	MemRequest     string `json:"memRequest,omitempty"`
	MemPercent     *int64 `json:"memPercent,omitempty"`
	MemRecommended string `json:"memRecommended,omitempty"`
}

// SizingReport groups pods into undersized/oversized/no-request buckets,
// along with the thresholds that produced the grouping.
type SizingReport struct {
	Undersized []SizingEntry `json:"undersized"`
	Oversized  []SizingEntry `json:"oversized"`
	NoRequests []string      `json:"noRequests"`

	OversizedThresholdPct  int64   `json:"oversizedThresholdPct"`
	UndersizedThresholdPct int64   `json:"undersizedThresholdPct"`
	TargetUtilization      float64 `json:"targetUtilization"`
}

// Report is the full result of one collection pass: node usage, pod usage,
// and the derived sizing report. This is the single value passed to output
// printers (text or JSON), and the natural response body if this ever grows
// into an HTTP API.
type Report struct {
	Timestamp time.Time    `json:"timestamp"`
	Nodes     []NodeUsage  `json:"nodes"`
	Pods      []PodUsage   `json:"pods"`
	Sizing    SizingReport `json:"sizing"`
}
