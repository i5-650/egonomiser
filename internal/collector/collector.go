// Package collector talks to the Kubernetes API server and metrics-server
// to gather node/pod usage data, and assembles it (together with a sizing
// report) into a single types.Report.
package collector

import (
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"

	"e-go-nomiser/internal/sizing"
	"e-go-nomiser/internal/types"
)

// Params bundles everything needed for one collection pass.
type Params struct {
	Namespace     string // namespace to query, or metav1.NamespaceAll
	AllNamespaces bool

	Sizing sizing.Params
}

// Collect gathers node usage, pod usage, and a derived sizing report into a
// single types.Report, timestamped at the moment it's built.
func Collect(ctx context.Context, clientset kubernetes.Interface, metricsClient metricsv.Interface, params Params) (types.Report, error) {
	report := types.Report{Timestamp: time.Now()}

	nodes, err := Nodes(ctx, clientset, metricsClient)
	if err != nil {
		return report, fmt.Errorf("fetching node metrics: %w", err)
	}
	report.Nodes = nodes

	pods, assessments, err := Pods(ctx, clientset, metricsClient, params.Namespace, params.AllNamespaces)
	if err != nil {
		return report, fmt.Errorf("fetching pod metrics: %w", err)
	}
	report.Pods = pods
	report.Sizing = sizing.Build(assessments, params.Sizing)

	return report, nil
}
