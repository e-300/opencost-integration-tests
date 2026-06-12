package schema

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/opencost/opencost-integration-tests/pkg/api"
)

func fetchRawEndpoint(t *testing.T, apiClient *api.API, path string, req api.AutocompleteRequest) map[string]any {
	t.Helper()

	status, body, err := apiClient.GetAutocompleteStatus(path, req)
	if err != nil {
		t.Fatalf("%s request failed: %v", path, err)
	}

	if status != http.StatusOK {
		t.Fatalf("%s returned HTTP %d: %s", path, status, strings.TrimSpace(string(body)))
	}

	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("%s returned non-JSON or malformed JSON: %v\nbody: %s", path, err, strings.TrimSpace(string(body)))
	}

	return parsed
}

func requireFields(t *testing.T, context string, obj map[string]any, fields []string) {
	t.Helper()

	for _, field := range fields {
		if _, ok := obj[field]; !ok {
			t.Errorf("%s missing required field %q", context, field)
		}
	}
}

func requireMap(t *testing.T, context string, value any) map[string]any {
	t.Helper()

	obj, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s expected object, got %T", context, value)
	}

	return obj
}

func requireArray(t *testing.T, context string, value any) []any {
	t.Helper()

	arr, ok := value.([]any)
	if !ok {
		t.Fatalf("%s expected array, got %T", context, value)
	}

	return arr
}

var allocationRequiredFields = []string{
	"name",
	"cpuCost",
	"ramCost",
	"gpuCost",
	"pvCost",
	"networkCost",
	"loadBalancerCost",
	"sharedCost",
	"externalCost",
	"totalCost",
	"minutes",
	"window",
	"properties",
}

var windowRequiredFields = []string{
	"start",
	"end",
}

var assetCommonRequiredFields = []string{
	"type",
	"properties",
	"window",
	"start",
	"end",
	"minutes",
	"totalCost",
}

var assetCommonPropertiesRequiredFields = []string{
	"category",
	"provider",
	"service",
}

var assetNodeRequiredFields = []string{
	"cpuCost",
	"ramCost",
	"gpuCost",
	"nodeType",
	"cpuCores",
	"ramBytes",
}

var assetNodePropertiesRequiredFields = []string{
	"name",
	"providerID",
}

var cloudCostItemRequiredFields = []string{
	"properties",
	"window",
	"netCost",
	"amortizedCost",
	"amortizedNetCost",
	"invoicedCost",
	"listCost",
}

var cloudCostPropertiesRequiredFields = []string{
	"provider",
	"accountID",
	"accountName",
	"invoiceEntityID",
	"invoiceEntityName",
	"service",
	"category",
}

var cloudCostCostObjectRequiredFields = []string{
	"cost",
	"kubernetesPercent",
}

func TestAllocationResponseSchemaStability(t *testing.T) {
	apiClient := api.NewAPI()

	resp := fetchRawEndpoint(t, apiClient, "/allocation", api.AutocompleteRequest{
		Window: "1h",
		// This assumes AutocompleteRequest.QueryString can encode these fields only if available.
		// If it cannot, use a different raw request helper or existing AllocationRequest path.
	})

	requireFields(t, "/allocation response", resp, []string{"code", "data"})

	data := requireArray(t, "/allocation data", resp["data"])
	if len(data) == 0 {
		t.Skip("/allocation returned no data sets")
	}

	for setIndex, rawSet := range data {
		set := requireMap(t, fmt.Sprintf("/allocation data[%d]", setIndex), rawSet)
		if len(set) == 0 {
			t.Skipf("/allocation data[%d] returned no allocation items", setIndex)
		}

		for itemName, rawItem := range set {
			item := requireMap(t, fmt.Sprintf("/allocation item %q", itemName), rawItem)

			requireFields(t, fmt.Sprintf("/allocation item %q", itemName), item, allocationRequiredFields)

			window := requireMap(t, fmt.Sprintf("/allocation item %q window", itemName), item["window"])
			requireFields(t, fmt.Sprintf("/allocation item %q window", itemName), window, windowRequiredFields)
		}
	}
}

func TestAssetsResponseSchemaStability(t *testing.T) {
	apiClient := api.NewAPI()

	resp := fetchRawEndpoint(t, apiClient, "/assets", api.AutocompleteRequest{
		Window: "1h",
	})

	requireFields(t, "/assets response", resp, []string{"code", "data"})

	data := requireMap(t, "/assets data", resp["data"])
	if len(data) == 0 {
		t.Skip("/assets returned no asset items")
	}

	for assetName, rawAsset := range data {
		itemContext := fmt.Sprintf("/assets item %q", assetName)
		asset := requireMap(t, itemContext, rawAsset)

		requireFields(t, itemContext, asset, assetCommonRequiredFields)

		window := requireMap(t, itemContext+" window", asset["window"])
		requireFields(t, itemContext+" window", window, windowRequiredFields)

		properties := requireMap(t, itemContext+" properties", asset["properties"])
		requireFields(t, itemContext+" properties", properties, assetCommonPropertiesRequiredFields)

		assetType, ok := asset["type"].(string)
		if !ok {
			t.Errorf("%s field \"type\" expected string, got %T", itemContext, asset["type"])
			continue
		}

		if assetType == "Node" {
			requireFields(t, itemContext, asset, assetNodeRequiredFields)
			requireFields(t, itemContext+" properties", properties, assetNodePropertiesRequiredFields)
		}
	}
}

func TestCloudCostResponseSchemaStability(t *testing.T) {
	apiClient := api.NewAPI()

	resp := fetchRawEndpoint(t, apiClient, "/cloudCost", api.AutocompleteRequest{
		Window: "1d",
	})

	requireFields(t, "/cloudCost response", resp, []string{"code", "data"})

	data := requireMap(t, "/cloudCost data", resp["data"])
	requireFields(t, "/cloudCost data", data, []string{"sets"})

	sets := requireArray(t, "/cloudCost data.sets", data["sets"])
	if len(sets) == 0 {
		t.Skip("/cloudCost returned no sets")
	}

	for setIndex, rawSet := range sets {
		set := requireMap(t, fmt.Sprintf("/cloudCost set[%d]", setIndex), rawSet)
		requireFields(t, fmt.Sprintf("/cloudCost set[%d]", setIndex), set, []string{"cloudCosts"})

		cloudCosts := requireMap(t, fmt.Sprintf("/cloudCost set[%d].cloudCosts", setIndex), set["cloudCosts"])
		if len(cloudCosts) == 0 {
			t.Skipf("/cloudCost set[%d] returned no cloud cost items", setIndex)
		}

		for itemName, rawItem := range cloudCosts {
			item := requireMap(t, fmt.Sprintf("/cloudCost item %q", itemName), rawItem)

			requireFields(t, fmt.Sprintf("/cloudCost item %q", itemName), item, cloudCostItemRequiredFields)

			properties := requireMap(t, fmt.Sprintf("/cloudCost item %q properties", itemName), item["properties"])
			requireFields(t, fmt.Sprintf("/cloudCost item %q properties", itemName), properties, cloudCostPropertiesRequiredFields)

			for _, costField := range []string{
				"netCost",
				"amortizedCost",
				"amortizedNetCost",
				"invoicedCost",
				"listCost",
			} {
				costObject := requireMap(t, fmt.Sprintf("/cloudCost item %q %s", itemName, costField), item[costField])
				requireFields(t, fmt.Sprintf("/cloudCost item %q %s", itemName, costField), costObject, cloudCostCostObjectRequiredFields)
			}
		}
	}
}