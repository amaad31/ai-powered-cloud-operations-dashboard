package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
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

	log.Println("collector started, polling every 15s")

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// Run once immediately, then on every tick.
	collectOnce(clientset, db, clusterID)
	for range ticker.C {
		collectOnce(clientset, db, clusterID)
	}
}

func collectOnce(clientset *kubernetes.Clientset, db *sql.DB, clusterID int) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("collector: failed to list pods: %v", err)
		return
	}

	for _, pod := range pods.Items {
		restartCount := int32(0)
		for _, cs := range pod.Status.ContainerStatuses {
			restartCount += cs.RestartCount
		}

		_, err := db.Exec(
			`INSERT INTO metrics (cluster_id, pod_name, namespace, restart_count, recorded_at)
			 VALUES ($1, $2, $3, $4, now())`,
			clusterID, pod.Name, pod.Namespace, restartCount,
		)
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
