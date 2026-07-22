package collector

import (
	"context"
	"sort"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"

	"e-go-nomiser/internal/format"
	"e-go-nomiser/internal/types"
)

// Nodes fetches CPU/memory usage per node alongside allocatable capacity,
// mirroring kubectl's node table but with the capacity baked in.
func Nodes(ctx context.Context, clientset kubernetes.Interface, metricsClient metricsv.Interface) ([]types.NodeUsage, error) {
	nodeMetricsList, err := metricsClient.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	allocatable := make(map[string]corev1.ResourceList, len(nodes.Items))
	for _, n := range nodes.Items {
		allocatable[n.Name] = n.Status.Allocatable
	}

	sort.Slice(nodeMetricsList.Items, func(i, j int) bool {
		return nodeMetricsList.Items[i].Name < nodeMetricsList.Items[j].Name
	})

	result := make([]types.NodeUsage, 0, len(nodeMetricsList.Items))
	for _, m := range nodeMetricsList.Items {
		cpuUsage := m.Usage[corev1.ResourceCPU]
		memUsage := m.Usage[corev1.ResourceMemory]

		alloc := allocatable[m.Name] // zero value ResourceList if node wasn't found
		cpuAlloc := alloc[corev1.ResourceCPU]
		memAlloc := alloc[corev1.ResourceMemory]

		nu := types.NodeUsage{
			Name:                m.Name,
			CPUUsedMilli:        cpuUsage.MilliValue(),
			CPUAllocatableMilli: cpuAlloc.MilliValue(),
			MemUsedBytes:        memUsage.Value(),
			MemAllocatableBytes: memAlloc.Value(),
		}
		if !cpuAlloc.IsZero() {
			v := format.PercentOf(cpuUsage, cpuAlloc)
			nu.CPUPercent = &v
		}
		if !memAlloc.IsZero() {
			v := format.PercentOf(memUsage, memAlloc)
			nu.MemPercent = &v
		}
		result = append(result, nu)
	}
	return result, nil
}
