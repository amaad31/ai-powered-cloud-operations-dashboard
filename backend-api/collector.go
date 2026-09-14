package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

// startCollector connects to the Kubernetes cluster pointed to by KUBECONFIG
// and periodically writes pod status into the metrics table.
// It runs forever in its own goroutine and never returns under normal operation.
func startCollector(db *sql.DB) {
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		log.Println("KUBECONFIG not set, collector will not start")
		return
	}

	clusterIDStr := os.Getenv("CLUSTER_ID")
	clusterID, err := strconv.Atoi(clusterIDStr)
	if err != nil {
		log.Printf("invalid or missing CLUSTER_ID, collector will not start: %v", err)
		return
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		log.Printf("failed to build kube config, collector will not start: %v", err)
		return
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("failed to create kubernetes client, collector will not start: %v", err)
		return
	}

	metricsClient, err := metricsclientset.NewForConfig(config)
	if err != nil {
		log.Printf("failed to create metrics client, collector will not start: %v", err)
		return
	}

	log.Println("collector started, polling every 15s")

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// Run once immediately, then on every tick.
	collectOnce(clientset, metricsClient, db, clusterID)
	for range ticker.C {
		collectOnce(clientset, metricsClient, db, clusterID)
	}
}

func collectOnce(clientset *kubernetes.Clientset, metricsClient *metricsclientset.Clientset, db *sql.DB, clusterID int) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("collector: failed to list pods: %v", err)
		return
	}

	// Fetch CPU/memory usage separately from metrics-server.
	// Build a lookup map so we can join it with the pod list below.
	podMetrics, err := metricsClient.MetricsV1beta1().PodMetricses("").List(ctx, metav1.ListOptions{})
	usage := make(map[string]struct {
		cpuMillicores int64
		memoryBytes   int64
	})
	if err != nil {
		log.Printf("collector: failed to fetch pod metrics (cpu/memory will be null): %v", err)
	} else {
		for _, pm := range podMetrics.Items {
			var cpu, mem int64
			for _, c := range pm.Containers {
				cpu += c.Usage.Cpu().MilliValue()
				mem += c.Usage.Memory().Value()
			}
			key := pm.Namespace + "/" + pm.Name
			usage[key] = struct {
				cpuMillicores int64
				memoryBytes   int64
			}{cpu, mem}
		}
	}

	for _, pod := range pods.Items {
		restartCount := int32(0)
		for _, cs := range pod.Status.ContainerStatuses {
			restartCount += cs.RestartCount
		}

		key := pod.Namespace + "/" + pod.Name
		u, hasUsage := usage[key]

		_, err := db.Exec(
			`INSERT INTO metrics (cluster_id, pod_name, namespace, cpu_millicores, memory_bytes, restart_count, recorded_at)
			 VALUES ($1, $2, $3, $4, $5, $6, now())`,
			clusterID, pod.Name, pod.Namespace,
			nullableInt64(hasUsage, u.cpuMillicores),
			nullableInt64(hasUsage, u.memoryBytes),
			restartCount,
		)

		var memBytes int64
		if hasUsage {
			memBytes = u.memoryBytes
		}
		checkAlerts(db, clusterID, pod.Name, restartCount, memBytes, hasUsage)

		if err != nil {
			log.Printf("collector: failed to insert metric for pod %s: %v", pod.Name, err)
			continue
		}

		if pod.Status.Phase != corev1.PodRunning {
			log.Printf("collector: pod %s/%s is in phase %s", pod.Namespace, pod.Name, pod.Status.Phase)
		}
	}

	log.Printf("collector: recorded metrics for %d pods", len(pods.Items))
}

// nullableInt64 returns nil (so Postgres stores NULL) when usage data wasn't
// available for this pod, instead of writing a misleading 0.
func nullableInt64(has bool, v int64) interface{} {
	if !has {
		return nil
	}
	return v
}

// checkAlerts evaluates simple thresholds for a pod and opens/resolves
// alerts accordingly. One open alert per (pod, alert_type) at a time.
func checkAlerts(db *sql.DB, clusterID int, podName string, restartCount int32, memoryBytes int64, hasMemory bool) {
	checkThreshold(db, clusterID, podName, "restart_spike",
		restartCount > 3,
		fmt.Sprintf("Pod has restarted %d times", restartCount))

	if hasMemory {
		const memoryThresholdBytes = 200 * 1024 * 1024 // 200Mi, placeholder value
		checkThreshold(db, clusterID, podName, "memory_threshold",
			memoryBytes > memoryThresholdBytes,
			fmt.Sprintf("Memory usage %.1f MB exceeds threshold", float64(memoryBytes)/1024/1024))
	}
}

// checkThreshold opens a new alert if breached and none is currently open,
// or resolves the existing open alert if the condition is no longer breached.
func checkThreshold(db *sql.DB, clusterID int, podName, alertType string, breached bool, message string) {
	var existingID int
	err := db.QueryRow(
		`SELECT id FROM alerts WHERE cluster_id = $1 AND pod_name = $2 AND alert_type = $3 AND resolved = false`,
		clusterID, podName, alertType,
	).Scan(&existingID)
	hasOpenAlert := err == nil

	if breached && !hasOpenAlert {
		_, err := db.Exec(
			`INSERT INTO alerts (cluster_id, pod_name, alert_type, message) VALUES ($1, $2, $3, $4)`,
			clusterID, podName, alertType, message,
		)
		if err != nil {
			log.Printf("checkThreshold: failed to insert alert: %v", err)
			return
		}
		log.Printf("ALERT opened: %s for pod %s — %s", alertType, podName, message)
	} else if !breached && hasOpenAlert {
		_, err := db.Exec(`UPDATE alerts SET resolved = true WHERE id = $1`, existingID)
		if err != nil {
			log.Printf("checkThreshold: failed to resolve alert: %v", err)
			return
		}
		log.Printf("ALERT resolved: %s for pod %s", alertType, podName)
	}
}
