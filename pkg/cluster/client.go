package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	EnvBrokerURL   = "OPENCOST_BROKER_URL"
	EnvBrokerToken = "OPENCOST_BROKER_TOKEN"

	defaultOpenCostDeployment = "opencost"

	pathHealthz = "/healthz"
	pathChaos   = "/v1/chaos"
	pathPods    = "/v1/pods"
	pathRestart = "/v1/restart"
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

type Pod struct {
	Name         string `json:"name"`
	Phase        string `json:"phase"`
	Ready        bool   `json:"ready"`
	RestartCount int    `json:"restartCount"`
}

type Deployment struct {
	Name               string `json:"name"`
	Ready              bool   `json:"ready"`
	Replicas           int32  `json:"replicas"`
	ReadyReplicas      int32  `json:"readyReplicas"`
	UpdatedReplicas    int32  `json:"updatedReplicas"`
	AvailableReplicas  int32  `json:"availableReplicas"`
	ObservedGeneration int64  `json:"observedGeneration"`
	Generation         int64  `json:"generation"`
}

type ChaosScenario struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Engine      string `json:"engine"`
}

type brokerError struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

type healthResponse struct {
	Status string `json:"status"`
}

type restartResponse struct {
	Status string `json:"status"`
}

type podsResponse struct {
	Pods []Pod `json:"pods"`
}

type chaosScenariosResponse struct {
	Scenarios []ChaosScenario `json:"scenarios"`
}

type chaosInjectResponse struct {
	Injected bool   `json:"injected"`
	Scenario string `json:"scenario"`
}

type chaosCleanupResponse struct {
	Deleted  bool   `json:"deleted"`
	Scenario string `json:"scenario"`
}

func NewClientFromEnv() (*Client, error) {
	baseURL := os.Getenv(EnvBrokerURL)
	if baseURL == "" {
		return nil, fmt.Errorf("%s is required", EnvBrokerURL)
	}

	token := os.Getenv(EnvBrokerToken)
	if token == "" {
		return nil, fmt.Errorf("%s is required", EnvBrokerToken)
	}

	return NewClient(baseURL, token), nil
}

func NewClient(baseURL string, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) WithHTTPClient(httpClient *http.Client) *Client {
	if httpClient != nil {
		c.httpClient = httpClient
	}
	return c
}

func (c *Client) Healthz(ctx context.Context) error {
	var response healthResponse
	if err := c.do(ctx, http.MethodGet, pathHealthz, false, nil, &response); err != nil {
		return err
	}
	if response.Status != "ok" {
		return fmt.Errorf("unexpected broker health status %q", response.Status)
	}
	return nil
}

func (c *Client) RestartOpenCost(ctx context.Context) error {
	var response restartResponse
	if err := c.do(ctx, http.MethodPost, pathRestart, true, nil, &response); err != nil {
		return err
	}
	if response.Status != "restart triggered" {
		return fmt.Errorf("unexpected restart status %q", response.Status)
	}
	return nil
}

func (c *Client) Pods(ctx context.Context) ([]Pod, error) {
	var response podsResponse
	if err := c.do(ctx, http.MethodGet, pathPods, true, nil, &response); err != nil {
		return nil, err
	}
	return response.Pods, nil
}

func (c *Client) Deployment(ctx context.Context, name string) (Deployment, error) {
	var response Deployment
	if err := c.do(ctx, http.MethodGet, deploymentPath(name), true, nil, &response); err != nil {
		return Deployment{}, err
	}
	return response, nil
}

func (c *Client) ChaosScenarios(ctx context.Context) ([]ChaosScenario, error) {
	var response chaosScenariosResponse
	if err := c.do(ctx, http.MethodGet, pathChaos, true, nil, &response); err != nil {
		return nil, err
	}
	return response.Scenarios, nil
}

func (c *Client) InjectChaos(ctx context.Context, scenario string) error {
	var response chaosInjectResponse
	if err := c.do(ctx, http.MethodPost, chaosScenarioPath(scenario), true, nil, &response); err != nil {
		return err
	}
	if !response.Injected || response.Scenario != scenario {
		return fmt.Errorf("unexpected chaos inject response: injected=%t scenario=%q", response.Injected, response.Scenario)
	}
	return nil
}

func (c *Client) CleanupChaos(ctx context.Context, scenario string) error {
	var response chaosCleanupResponse
	if err := c.do(ctx, http.MethodDelete, chaosScenarioPath(scenario), true, nil, &response); err != nil {
		return err
	}
	if !response.Deleted || response.Scenario != scenario {
		return fmt.Errorf("unexpected chaos cleanup response: deleted=%t scenario=%q", response.Deleted, response.Scenario)
	}
	return nil
}

func (c *Client) WaitForOpenCostReady(ctx context.Context, interval time.Duration) ([]Pod, error) {
	if _, err := c.WaitForDeploymentReady(ctx, defaultOpenCostDeployment, interval); err != nil {
		return nil, err
	}
	return c.WaitForPodsReady(ctx, interval)
}

func (c *Client) WaitForPodsReady(ctx context.Context, interval time.Duration) ([]Pod, error) {
	if interval <= 0 {
		interval = time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		pods, err := c.Pods(ctx)
		if err != nil {
			return nil, err
		}
		if allPodsReady(pods) {
			return pods, nil
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("waiting for OpenCost pods ready: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (c *Client) WaitForDeploymentReady(ctx context.Context, name string, interval time.Duration) (Deployment, error) {
	if interval <= 0 {
		interval = time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		deployment, err := c.Deployment(ctx, name)
		if err != nil {
			return Deployment{}, err
		}
		if deployment.Ready {
			return deployment, nil
		}

		select {
		case <-ctx.Done():
			return Deployment{}, fmt.Errorf("waiting for deployment %q ready: %w", name, ctx.Err())
		case <-ticker.C:
		}
	}
}

func (c *Client) do(ctx context.Context, method string, path string, authed bool, requestBody any, responseBody any) error {
	var body io.Reader
	if requestBody != nil {
		raw, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("marshal broker request: %w", err)
		}
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.url(path), body)
	if err != nil {
		return fmt.Errorf("create broker request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if authed {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send broker request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read broker response %s %s: %w", method, path, err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return brokerRequestError(method, path, resp.StatusCode, raw)
	}

	if responseBody == nil {
		return nil
	}
	if err := json.Unmarshal(raw, responseBody); err != nil {
		return fmt.Errorf("decode broker response %s %s: %w body=%s", method, path, err, strings.TrimSpace(string(raw)))
	}
	return nil
}

func (c *Client) url(path string) string {
	return c.baseURL + "/" + strings.TrimLeft(path, "/")
}

func chaosScenarioPath(scenario string) string {
	return pathChaos + "/" + strings.TrimLeft(scenario, "/")
}

func deploymentPath(name string) string {
	return "/v1/deployments/" + strings.TrimLeft(name, "/")
}

func brokerRequestError(method string, path string, statusCode int, raw []byte) error {
	body := strings.TrimSpace(string(raw))
	var brokerErr brokerError
	if err := json.Unmarshal(raw, &brokerErr); err == nil && brokerErr.Error != "" {
		if brokerErr.Code != "" {
			return fmt.Errorf("broker request %s %s failed: status=%d code=%s error=%s", method, path, statusCode, brokerErr.Code, brokerErr.Error)
		}
		return fmt.Errorf("broker request %s %s failed: status=%d error=%s", method, path, statusCode, brokerErr.Error)
	}
	return fmt.Errorf("broker request %s %s failed: status=%d body=%s", method, path, statusCode, body)
}

func allPodsReady(pods []Pod) bool {
	if len(pods) == 0 {
		return false
	}
	for _, pod := range pods {
		if !pod.Ready {
			return false
		}
	}
	return true
}
