// Package output renders a types.Report either as human-readable text
// (tables + sizing summary) or as JSON.
package output

import (
	"fmt"
	"io"
	"text/tabwriter"

	"e-go-nomiser/internal/format"
	"e-go-nomiser/internal/types"
)

// Text writes the full report to w as human-readable tables and a sizing
// summary, matching the classic CLI output of this tool.
func Text(w io.Writer, report types.Report) error {
	fmt.Fprintln(w, "=== Node usage vs allocatable (like `kubectl top nodes` + capacity) ===")
	if err := printNodes(w, report.Nodes); err != nil {
		return err
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "=== Pod usage vs requested (like `kubectl top pods` + requests) ===")
	if err := printPods(w, report.Pods); err != nil {
		return err
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "=== Sizing report ===")
	printSizing(w, report.Sizing)

	return nil
}

func printNodes(w io.Writer, nodes []types.NodeUsage) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tCPU (used/allocatable)\tMEMORY (used/allocatable)")
	for _, n := range nodes {
		fmt.Fprintf(tw, "%s\t%s\t%s\n",
			n.Name,
			format.RatioStringRaw(n.CPUUsedMilli, n.CPUAllocatableMilli, n.CPUPercent, format.CPUMilli),
			format.RatioStringRaw(n.MemUsedBytes, n.MemAllocatableBytes, n.MemPercent, format.MemoryBytes),
		)
	}
	return tw.Flush()
}

func printPods(w io.Writer, pods []types.PodUsage) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAMESPACE\tPOD\tCPU (used/requested)\tMEMORY (used/requested)")
	for _, p := range pods {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			p.Namespace,
			p.Name,
			format.RatioStringRaw(p.CPUUsedMilli, p.CPURequestMilli, p.CPUPercent, format.CPUMilli),
			format.RatioStringRaw(p.MemUsedBytes, p.MemRequestBytes, p.MemPercent, format.MemoryBytes),
		)
	}
	return tw.Flush()
}

func printSizing(w io.Writer, s types.SizingReport) {
	fmt.Fprintf(w, "Undersized (using >%d%% of a request):\n", s.UndersizedThresholdPct)
	printSizingEntries(w, s.Undersized)

	fmt.Fprintf(w, "\nOversized (using <%d%% of every request it has):\n", s.OversizedThresholdPct)
	printSizingEntries(w, s.Oversized)

	if len(s.NoRequests) > 0 {
		fmt.Fprintln(w, "\nNo requests set (can't be sized against anything):")
		for _, l := range s.NoRequests {
			fmt.Fprintf(w, "  - %s\n", l)
		}
	}

	fmt.Fprintf(w, "\nRecommendations target ~%.0f%% utilization of the new request. This reflects a single "+
		"snapshot in time — confirm with usage history (e.g. metrics over days, covering peak load) "+
		"before resizing anything.\n", s.TargetUtilization*100)
}

func printSizingEntries(w io.Writer, entries []types.SizingEntry) {
	if len(entries) == 0 {
		fmt.Fprintln(w, "  (none)")
		return
	}
	for _, e := range entries {
		fmt.Fprintf(w, "  - %s/%s\n", e.Namespace, e.Name)
		if e.CPURequest != "" {
			fmt.Fprintf(w, "      CPU:    %s used / %s requested (%d%%) -> recommend ~%s\n",
				e.CPUUsage, e.CPURequest, *e.CPUPercent, e.CPURecommended)
		}
		if e.MemRequest != "" {
			fmt.Fprintf(w, "      MEMORY: %s used / %s requested (%d%%) -> recommend ~%s\n",
				e.MemUsage, e.MemRequest, *e.MemPercent, e.MemRecommended)
		}
	}
}
