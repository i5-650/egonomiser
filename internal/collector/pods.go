package collector

import (
	"context"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"

	"e-go-nomiser/internal/format"
	"e-go-nomiser/internal/types"
)

// kubeSystemNamespace is always excluded in all-namespaces mode: it's
// managed by the cluster/cloud provider, not something you can right-size.
const kubeSystemNamespace = "kube-system"

// Pods fetches CPU/memory usage per pod (summed across containers) against
// its requested amount, for a given namespace, or across all namespaces if
// ns == metav1.NamespaceAll. "Requested" is the sum of container resource
// requests, which is what the scheduler actually reserves against node
// allocatable capacity — this is a more meaningful baseline than limits for
// "am I over/under provisioned" questions. In all-namespaces mode,
// kube-system is skipped since those pods aren't yours to tune.
//
// Returns both the per-pod usage (for display) and the raw assessments
// used to build the sizing report.
func Pods(ctx context.Context, clientset kubernetes.Interface, metricsClient metricsv.Interface, ns string, allNamespaces bool) ([]types.PodUsage, []types.PodAssessment, error) {
	podMetricsList, err := metricsClient.MetricsV1beta1().PodMetricses(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}

	pods, err := clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}
	type podKey struct{ namespace, name string }
	requested := make(map[podKey]corev1.ResourceList, len(pods.Items))
	for _, p := range pods.Items {
		if allNamespaces && p.Namespace == kubeSystemNamespace {
			continue
		}
		var cpuReq, memReq resource.Quantity
		// Regular containers run concurrently, so their requests are additive.
		for _, c := range p.Spec.Containers {
			cpuReq.Add(*c.Resources.Requests.Cpu())
			memReq.Add(*c.Resources.Requests.Memory())
		}
		// Init containers run sequentially before regular ones, so they only
		// matter if a single init container asks for more than the combined
		// regular-container total (matches the scheduler's effective request).
		for _, c := range p.Spec.InitContainers {
			if initCPU := c.Resources.Requests.Cpu(); initCPU.Cmp(cpuReq) > 0 {
				cpuReq = *initCPU
			}
			if initMem := c.Resources.Requests.Memory(); initMem.Cmp(memReq) > 0 {
				memReq = *initMem
			}
		}
		requested[podKey{p.Namespace, p.Name}] = corev1.ResourceList{
			corev1.ResourceCPU:    cpuReq,
			corev1.ResourceMemory: memReq,
		}
	}

	// Filter out kube-system before sorting/reporting too, since its metrics
	// were fetched in the same all-namespaces List call.
	items := podMetricsList.Items[:0]
	for _, m := range podMetricsList.Items {
		if allNamespaces && m.Namespace == kubeSystemNamespace {
			continue
		}
		items = append(items, m)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	usages := make([]types.PodUsage, 0, len(items))
	assessments := make([]types.PodAssessment, 0, len(items))

	for _, m := range items {
		var cpuUsage, memUsage resource.Quantity
		for _, c := range m.Containers {
			cpuUsage.Add(c.Usage[corev1.ResourceCPU])
			memUsage.Add(c.Usage[corev1.ResourceMemory])
		}

		req := requested[podKey{m.Namespace, m.Name}] // zero value if pod spec wasn't found
		cpuReq := req[corev1.ResourceCPU]
		memReq := req[corev1.ResourceMemory]

		pu := types.PodUsage{
			Namespace:    m.Namespace,
			Name:         m.Name,
			CPUUsedMilli: cpuUsage.MilliValue(),
			MemUsedBytes: memUsage.Value(),
		}
		a := types.PodAssessment{
			Namespace:     m.Namespace,
			Name:          m.Name,
			CPUUsageMilli: cpuUsage.MilliValue(),
			MemUsageBytes: memUsage.Value(),
		}
		if !cpuReq.IsZero() {
			pu.CPURequestMilli = cpuReq.MilliValue()
			pu.HasCPURequest = true
			v := format.PercentOf(cpuUsage, cpuReq)
			pu.CPUPercent = &v

			a.CPURequestMilli = cpuReq.MilliValue()
			a.HasCPURequest = true
		}
		if !memReq.IsZero() {
			pu.MemRequestBytes = memReq.Value()
			pu.HasMemRequest = true
			v := format.PercentOf(memUsage, memReq)
			pu.MemPercent = &v

			a.MemRequestBytes = memReq.Value()
			a.HasMemRequest = true
		}

		usages = append(usages, pu)
		assessments = append(assessments, a)
	}

	return usages, assessments, nil
}
