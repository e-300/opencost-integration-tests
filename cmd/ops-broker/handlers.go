package main

import (
	"encoding/json"
	"net/http"
)

// newMux wires the frozen HTTP contract. Each authed route is a named button;
// none accept a target from the caller — targets are fixed by broker config.
//
//	GET  /healthz   — unauthenticated liveness
//	GET  /v1/pods   — list OpenCost pods (wait-for-ready)
//	POST /v1/restart — trigger rolling restart of OpenCost
func newMux(cfg Config, k8s *K8sClient) *http.ServeMux {
	mux := http.NewServeMux()

	// Unauthenticated liveness probe.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /v1/pods", requireToken(cfg.AuthToken,
		func(w http.ResponseWriter, r *http.Request) {
			pods, err := k8s.PodStatus(r.Context())
			if err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"pods": pods})
		}))

	mux.HandleFunc("POST /v1/restart", requireToken(cfg.AuthToken,
		func(w http.ResponseWriter, r *http.Request) {
			if err := k8s.RestartOpenCost(r.Context()); err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "restart triggered"})
		}))

	return mux
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
