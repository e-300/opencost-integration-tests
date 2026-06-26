package main

import (
	"fmt"
	"os"
)

// Config is the broker's runtime configuration, all from the environment.
type Config struct {
	// AuthToken is the bearer token callers (tests) must present. This is the
	// broker's OWN auth — separate from the k8s service-account token.
	AuthToken string
	// Addr is the listen address, e.g. ":8080".
	Addr string
	// Namespace is the namespace the broker is allowed to operate in.
	Namespace string
	// OpenCostDeployment is the deployment name the restart op targets.
	OpenCostDeployment string
	// OpenCostSelector is the label selector used to list OpenCost pods.
	OpenCostSelector string
	// PrometheusNamespace is the namespace used by Prometheus chaos scenarios.
	PrometheusNamespace string
	// PrometheusSelector is the label selector used by Prometheus chaos scenarios.
	PrometheusSelector string
	// ChaosNamespace is where broker-owned Chaos Mesh resources are created.
	ChaosNamespace string
	// Kubeconfig, if set, runs the broker out-of-cluster (local dev). Empty
	// means in-cluster (rest.InClusterConfig).
	Kubeconfig string
}

func LoadConfig() (Config, error) {
	c := Config{
		AuthToken:           os.Getenv("BROKER_AUTH_TOKEN"),
		Addr:                getEnv("BROKER_ADDR", ":8080"),
		Namespace:           getEnv("BROKER_NAMESPACE", "opencost"),
		OpenCostDeployment:  getEnv("BROKER_OPENCOST_DEPLOYMENT", "opencost"),
		OpenCostSelector:    getEnv("BROKER_OPENCOST_SELECTOR", "app.kubernetes.io/name=opencost"),
		PrometheusNamespace: getEnv("BROKER_PROMETHEUS_NAMESPACE", "prometheus"),
		PrometheusSelector:  getEnv("BROKER_PROMETHEUS_SELECTOR", "app.kubernetes.io/name=prometheus"),
		ChaosNamespace:      getEnv("BROKER_CHAOS_NAMESPACE", "opencost"),
		Kubeconfig:          os.Getenv("KUBECONFIG"),
	}
	if c.AuthToken == "" {
		return Config{}, fmt.Errorf("BROKER_AUTH_TOKEN is required")
	}
	return c, nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
