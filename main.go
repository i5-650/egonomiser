package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"

	"e-go-nomiser/internal/collector"
	"e-go-nomiser/internal/k8sconfig"
	"e-go-nomiser/internal/output"
	"e-go-nomiser/internal/sizing"
)

func main() {
	var kubeconfig, namespace, outputFormat string
	var allNamespaces bool
	var oversizedPct, undersizedPct int64
	var targetUtilization float64
	var watchInterval time.Duration

	flag.StringVar(&kubeconfig, "kubeconfig", k8sconfig.DefaultKubeconfigPath(), "path to kubeconfig file (empty for in-cluster)")
	flag.StringVar(&namespace, "namespace", "default", "namespace to query pods in")
	flag.BoolVar(&allNamespaces, "all-namespaces", false, "query pods across all namespaces (kube-system is always skipped in this mode)")
	flag.Int64Var(&oversizedPct, "oversized-threshold", 20, "flag a pod as oversized if usage stays below this %% of its request (for every resource that has a request set)")
	flag.Int64Var(&undersizedPct, "undersized-threshold", 100, "flag a pod as undersized if usage exceeds this %% of its request for any resource")
	flag.Float64Var(&targetUtilization, "target-utilization", 0.6, "recommended requests aim for usage to sit at this fraction of the request (e.g. 0.6 = 60%%)")
	flag.StringVar(&outputFormat, "output", "text", `output format: "text" or "json"`)
	flag.DurationVar(&watchInterval, "watch", 0, "if set, re-run the report every interval (e.g. 30s, 5m); each result is printed once its interval elapses, repeating until interrupted (0 runs once and exits)")
	flag.Parse()

	if outputFormat != "text" && outputFormat != "json" {
		fmt.Fprintf(os.Stderr, "invalid -output %q: must be \"text\" or \"json\"\n", outputFormat)
		os.Exit(1)
	}

	config, err := k8sconfig.Load(kubeconfig)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error building kubeconfig:", err)
		os.Exit(1)
	}

	// Regular client-go clientset -> used for object metadata (node/pod lists, capacity, etc.)
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error creating clientset:", err)
		os.Exit(1)
	}

	// Metrics clientset -> talks to metrics.k8s.io, served by metrics-server
	metricsClient, err := metricsv.NewForConfig(config)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error creating metrics clientset:", err)
		os.Exit(1)
	}

	ns := namespace
	if allNamespaces {
		ns = metav1.NamespaceAll
	}

	params := collector.Params{
		Namespace:     ns,
		AllNamespaces: allNamespaces,
		Sizing: sizing.Params{
			OversizedPct:      oversizedPct,
			UndersizedPct:     undersizedPct,
			TargetUtilization: targetUtilization,
		},
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runOnce := func() error {
		report, err := collector.Collect(ctx, clientset, metricsClient, params)
		if err != nil {
			return err
		}
		if outputFormat == "json" {
			return output.JSON(os.Stdout, report)
		}
		return output.Text(os.Stdout, report)
	}

	if watchInterval <= 0 {
		if err := runOnce(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}

	fmt.Fprintf(os.Stderr, "watching every %s (press Ctrl+C to stop)...\n", watchInterval)
	ticker := time.NewTicker(watchInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := runOnce(); err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
			}
		}
	}
}
