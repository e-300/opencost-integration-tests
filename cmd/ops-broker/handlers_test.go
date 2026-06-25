package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestChaosScenariosEndpoint(t *testing.T) {
	mux := newMux(Config{AuthToken: "test-token"}, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/chaos", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var response struct {
		Scenarios []ChaosScenario `json:"scenarios"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Scenarios) != 4 {
		t.Fatalf("len(scenarios) = %d, want 4", len(response.Scenarios))
	}
}

func TestDeploymentEndpointReturnsTrimmedReadiness(t *testing.T) {
	k8s := &K8sClient{
		client:    fake.NewSimpleClientset(readyDeployment()),
		namespace: "opencost",
		deploy:    "opencost",
	}
	mux := newMux(Config{AuthToken: "test-token"}, k8s)

	req := httptest.NewRequest(http.MethodGet, "/v1/deployments/opencost", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var response DeploymentInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Ready || response.Name != "opencost" || response.Replicas != 1 {
		t.Fatalf("unexpected deployment response: %+v", response)
	}
}

func TestDeploymentEndpointRejectsUnallowlistedDeployment(t *testing.T) {
	k8s := &K8sClient{
		client:    fake.NewSimpleClientset(readyDeployment()),
		namespace: "opencost",
		deploy:    "opencost",
	}
	mux := newMux(Config{AuthToken: "test-token"}, k8s)

	req := httptest.NewRequest(http.MethodGet, "/v1/deployments/prometheus", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestAuthedEndpointRejectsMissingTokenWithJSON(t *testing.T) {
	mux := newMux(Config{AuthToken: "test-token"}, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/chaos", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	var response map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	if response["error"] != "unauthorized" {
		t.Fatalf("error = %q, want unauthorized", response["error"])
	}
}

func readyDeployment() *appsv1.Deployment {
	replicas := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "opencost",
			Namespace:  "opencost",
			Generation: 3,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas:      1,
			UpdatedReplicas:    1,
			AvailableReplicas:  1,
			ObservedGeneration: 3,
		},
	}
}
