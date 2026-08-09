package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq"
)

type healthResponse struct {
	Status string `json:"status"`
	DB     string `json:"db"`
}

var db *sql.DB

type metricRow struct {
	PodName      string `json:"pod_name"`
	Namespace    string `json:"namespace"`
	RestartCount int    `json:"restart_count"`
	RecordedAt   string `json:"recorded_at"`
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`
		SELECT pod_name, namespace, restart_count, recorded_at
		FROM metrics
		ORDER BY recorded_at DESC
		LIMIT 50
	`)
	if err != nil {
		http.Error(w, "failed to query metrics", http.StatusInternalServerError)
		log.Printf("metrics query failed: %v", err)
		return
	}
	defer rows.Close()

	var results []metricRow
	for rows.Next() {
		var m metricRow
		if err := rows.Scan(&m.PodName, &m.Namespace, &m.RestartCount, &m.RecordedAt); err != nil {
			http.Error(w, "failed to read metrics", http.StatusInternalServerError)
			log.Printf("metrics scan failed: %v", err)
			return
		}
		results = append(results, m)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	resp := healthResponse{Status: "ok", DB: "connected"}

	if err := db.Ping(); err != nil {
		resp.DB = "unreachable"
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/cloudops?sslmode=disable"
	}

	var err error
	db, err = sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("failed to open db connection: %v", err)
	}
	defer db.Close()

	go startCollector(db)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/metrics", metricsHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("backend-api listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
