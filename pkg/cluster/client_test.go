package cluster

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientHealthzDoesNotSendAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathHealthz {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("Authorization header = %q, want empty", got)
		}
		writeJSON(t, w, healthResponse{Status: "ok"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	if err := client.Healthz(context.Background()); err != nil {
		t.Fatalf("Healthz() error = %v", err)
	}
}

func TestClientRestartOpenCostSendsBearerToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathRestart {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization header = %q, want bearer token", got)
		}
		writeJSON(t, w, restartResponse{Status: "restart triggered"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	if err := client.RestartOpenCost(context.Background()); err != nil {
		t.Fatalf("RestartOpenCost() error = %v", err)
	}
}

func TestClientPodsDecodesTrimmedPodList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathPods {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization header = %q, want bearer token", got)
		}
		writeJSON(t, w, podsResponse{
			Pods: []Pod{
				{Name: "opencost-abc", Phase: "Running", Ready: true, RestartCount: 1},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	pods, err := client.Pods(context.Background())
	if err != nil {
		t.Fatalf("Pods() error = %v", err)
	}
	if len(pods) != 1 {
		t.Fatalf("len(pods) = %d, want 1", len(pods))
	}
	if pods[0].Name != "opencost-abc" || pods[0].Phase != "Running" || !pods[0].Ready || pods[0].RestartCount != 1 {
		t.Fatalf("unexpected pod: %+v", pods[0])
	}
}

func TestClientDeploymentDecodesReadiness(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != deploymentPath("opencost") {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization header = %q, want bearer token", got)
		}
		writeJSON(t, w, Deployment{
			Name:               "opencost",
			Ready:              true,
			Replicas:           1,
			ReadyReplicas:      1,
			UpdatedReplicas:    1,
			AvailableReplicas:  1,
			ObservedGeneration: 3,
			Generation:         3,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	deployment, err := client.Deployment(context.Background(), "opencost")
	if err != nil {
		t.Fatalf("Deployment() error = %v", err)
	}
	if !deployment.Ready || deployment.Name != "opencost" || deployment.Replicas != 1 {
		t.Fatalf("unexpected deployment: %+v", deployment)
	}
}

func TestClientChaosScenariosDecodesScenarioList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathChaos {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization header = %q, want bearer token", got)
		}
		writeJSON(t, w, chaosScenariosResponse{
			Scenarios: []ChaosScenario{
				{ID: "kill-opencost", Description: "Kill OpenCost", Engine: "chaos-mesh"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	scenarios, err := client.ChaosScenarios(context.Background())
	if err != nil {
		t.Fatalf("ChaosScenarios() error = %v", err)
	}
	if len(scenarios) != 1 {
		t.Fatalf("len(scenarios) = %d, want 1", len(scenarios))
	}
	if scenarios[0].ID != "kill-opencost" || scenarios[0].Engine != "chaos-mesh" {
		t.Fatalf("unexpected scenario: %+v", scenarios[0])
	}
}

func TestClientInjectAndCleanupChaos(t *testing.T) {
	requests := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization header = %q, want bearer token", got)
		}

		requests[r.Method+" "+r.URL.Path]++
		switch {
		case r.Method == http.MethodPost && r.URL.Path == chaosScenarioPath("kill-opencost"):
			writeJSON(t, w, chaosInjectResponse{Injected: true, Scenario: "kill-opencost"})
		case r.Method == http.MethodDelete && r.URL.Path == chaosScenarioPath("kill-opencost"):
			writeJSON(t, w, chaosCleanupResponse{Deleted: true, Scenario: "kill-opencost"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	if err := client.InjectChaos(context.Background(), "kill-opencost"); err != nil {
		t.Fatalf("InjectChaos() error = %v", err)
	}
	if err := client.CleanupChaos(context.Background(), "kill-opencost"); err != nil {
		t.Fatalf("CleanupChaos() error = %v", err)
	}

	if requests[http.MethodPost+" "+chaosScenarioPath("kill-opencost")] != 1 {
		t.Fatalf("expected one inject request, got %d", requests[http.MethodPost+" "+chaosScenarioPath("kill-opencost")])
	}
	if requests[http.MethodDelete+" "+chaosScenarioPath("kill-opencost")] != 1 {
		t.Fatalf("expected one cleanup request, got %d", requests[http.MethodDelete+" "+chaosScenarioPath("kill-opencost")])
	}
}

func TestClientReportsBrokerJSONError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		writeJSON(t, w, brokerError{Error: "listing pods: unavailable"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	_, err := client.Pods(context.Background())
	if err == nil {
		t.Fatal("Pods() error = nil, want broker error")
	}
}

func TestNewClientFromEnv(t *testing.T) {
	t.Setenv(EnvBrokerURL, "http://broker.example.test/")
	t.Setenv(EnvBrokerToken, "test-token")

	client, err := NewClientFromEnv()
	if err != nil {
		t.Fatalf("NewClientFromEnv() error = %v", err)
	}
	if client.baseURL != "http://broker.example.test" {
		t.Fatalf("baseURL = %q, want trimmed URL", client.baseURL)
	}
	if client.token != "test-token" {
		t.Fatalf("token = %q, want test-token", client.token)
	}
}

func TestWaitForOpenCostReady(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case deploymentPath(defaultOpenCostDeployment):
			writeJSON(t, w, Deployment{Name: defaultOpenCostDeployment, Ready: true})
		case pathPods:
			requests++
			if requests == 1 {
				writeJSON(t, w, podsResponse{
					Pods: []Pod{{Name: "opencost-abc", Phase: "Running", Ready: false}},
				})
				return
			}
			writeJSON(t, w, podsResponse{
				Pods: []Pod{{Name: "opencost-abc", Phase: "Running", Ready: true}},
			})
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	pods, err := client.WaitForOpenCostReady(ctx, time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForOpenCostReady() error = %v", err)
	}
	if len(pods) != 1 || !pods[0].Ready {
		t.Fatalf("pods = %+v, want one ready pod", pods)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
}

func TestWaitForDeploymentReady(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != deploymentPath("opencost") {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if requests == 1 {
			writeJSON(t, w, Deployment{Name: "opencost", Ready: false})
			return
		}
		writeJSON(t, w, Deployment{Name: "opencost", Ready: true})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	deployment, err := client.WaitForDeploymentReady(ctx, "opencost", time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForDeploymentReady() error = %v", err)
	}
	if !deployment.Ready {
		t.Fatalf("deployment = %+v, want ready", deployment)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("write JSON: %v", err)
	}
}
