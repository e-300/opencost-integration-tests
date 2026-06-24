package main

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// K8sClient is a connection to the cluster the broker pod already runs in
// (in-cluster SA token, or KUBECONFIG for local dev). It does NOT create a
// cluster — it just talks to the existing one through a scoped clientset.
type K8sClient struct {
	client    kubernetes.Interface
	namespace string
	deploy    string
	selector  string
}

func NewK8sClient(cfg Config) (*K8sClient, error) {
	var restCfg *rest.Config
	var err error
	if cfg.Kubeconfig != "" {
		// Local dev: talk to the cluster in KUBECONFIG.
		restCfg, err = clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
	} else {
		// In-cluster: reads the auto-mounted SA token + CA + API host.
		restCfg, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, fmt.Errorf("building k8s rest config: %w", err)
	}

	client, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("building k8s client: %w", err)
	}

	return &K8sClient{
		client:    client,
		namespace: cfg.Namespace,
		deploy:    cfg.OpenCostDeployment,
		selector:  cfg.OpenCostSelector,
	}, nil
}

// RestartOpenCost triggers a rolling restart of the OpenCost deployment by
// stamping the pod-template restart annotation — the same mechanism as
// `kubectl rollout restart`. The target is fixed by config, never the caller.
func (c *K8sClient) RestartOpenCost(ctx context.Context) error {
	patch := fmt.Sprintf(
		`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":%q}}}}}`,
		time.Now().UTC().Format(time.RFC3339),
	)
	_, err := c.client.AppsV1().Deployments(c.namespace).Patch(
		ctx, c.deploy, types.StrategicMergePatchType, []byte(patch), metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("restarting deployment %s/%s: %w", c.namespace, c.deploy, err)
	}
	return nil
}

// PodInfo is the trimmed pod view the broker returns (not raw k8s objects).
type PodInfo struct {
	Name         string `json:"name"`
	Phase        string `json:"phase"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restartCount"`
}

// PodStatus lists OpenCost pods so tests can wait for readiness after a restart.
func (c *K8sClient) PodStatus(ctx context.Context) ([]PodInfo, error) {
	pods, err := c.client.CoreV1().Pods(c.namespace).List(
		ctx, metav1.ListOptions{LabelSelector: c.selector},
	)
	if err != nil {
		return nil, fmt.Errorf("listing pods in %s: %w", c.namespace, err)
	}

	out := make([]PodInfo, 0, len(pods.Items))
	for _, p := range pods.Items {
		out = append(out, PodInfo{
			Name:         p.Name,
			Phase:        string(p.Status.Phase),
			Ready:        podReady(p),
			RestartCount: totalRestarts(p),
		})
	}
	return out, nil
}

func podReady(p corev1.Pod) bool {
	for _, cond := range p.Status.Conditions {
		if cond.Type == corev1.PodReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

func totalRestarts(p corev1.Pod) int32 {
	var n int32
	for _, cs := range p.Status.ContainerStatuses {
		n += cs.RestartCount
	}
	return n
}
