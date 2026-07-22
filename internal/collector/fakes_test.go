package collector

import (
	"k8s.io/apimachinery/pkg/runtime"
	ktesting "k8s.io/client-go/testing"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
	fakemetrics "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

// newMetricsClient builds a fake metrics clientset that serves the given
// node/pod metrics on List calls.
//
// It intentionally does NOT pass objects to fakemetrics.NewSimpleClientset,
// because that client's default object tracker guesses each object's REST
// resource name from its Go type (e.g. "nodemetricses"), which doesn't
// match the real metrics.k8s.io API's resource names ("nodes"/"pods" — the
// same names the core API uses, by design). Reactors sidestep that mismatch
// by answering "list" calls directly.
func newMetricsClient(nodes []metricsv1beta1.NodeMetrics, pods []metricsv1beta1.PodMetrics) metricsv.Interface {
	cs := fakemetrics.NewSimpleClientset()

	cs.PrependReactor("list", "nodes", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, &metricsv1beta1.NodeMetricsList{Items: nodes}, nil
	})
	cs.PrependReactor("list", "pods", func(action ktesting.Action) (bool, runtime.Object, error) {
		ns := action.(ktesting.ListAction).GetNamespace()
		if ns == "" {
			return true, &metricsv1beta1.PodMetricsList{Items: pods}, nil
		}
		var filtered []metricsv1beta1.PodMetrics
		for _, p := range pods {
			if p.Namespace == ns {
				filtered = append(filtered, p)
			}
		}
		return true, &metricsv1beta1.PodMetricsList{Items: filtered}, nil
	})

	return cs
}
