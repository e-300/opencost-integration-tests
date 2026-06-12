package allocation
// TODO: Add cost adjustments if needed, e.g. for shared costs or idle costs, 
// depending on the allocation request parameters. This may require passing the 
// request parameters down to the component cost summation function.
import (
	"math"
	"testing"

	"github.com/opencost/opencost-integration-tests/pkg/api"
)

const (
	allocationCostAbsoluteEpsilon = 0.01  // one cent
	allocationCostRelativeEpsilon = 0.001 // 0.1%
)

// This test verifies that each returned allocation item is internally consistent.
// It does not query Prometheus or independently recompute costs. It only checks
// that totalCost equals the component cost fields returned on the same item.
func TestAllocationTotalCostMatchesComponents(t *testing.T) {
	apiClient := api.NewAPI()

	aggregates := []string{"namespace", "pod", "container"}

	for _, aggregate := range aggregates {
		t.Run(aggregate, func(t *testing.T) {
			resp, err := apiClient.GetAllocation(api.AllocationRequest{
				Window:      "1h",
				Aggregate:   aggregate,
				Accumulate:  "true",
				IncludeIdle: "true",
			})
			if err != nil {
				t.Fatalf("allocation API request failed for aggregate=%s: %v", aggregate, err)
			}

			if resp.Code != 200 {
				t.Fatalf("allocation API returned code=%d for aggregate=%s", resp.Code, aggregate)
			}

			if len(resp.Data) == 0 {
				t.Skipf("allocation API returned no data sets for aggregate=%s", aggregate)
			}

			for setIndex, allocationSet := range resp.Data {
				if len(allocationSet) == 0 {
					t.Skipf("allocation API returned empty allocation set at data[%d] for aggregate=%s", setIndex, aggregate)
				}

				for name, item := range allocationSet {
					componentSum := allocationComponentCost(item)

					if !withinCostTolerance(componentSum, item.TotalCost) {
						diff := math.Abs(componentSum - item.TotalCost)

						t.Errorf(
							"aggregate=%s data[%d] item=%q totalCost mismatch: components=%.6f totalCost=%.6f diff=%.6f cpu=%.6f ram=%.6f gpu=%.6f network=%.6f loadBalancer=%.6f shared=%.6f",
							aggregate,
							setIndex,
							name,
							componentSum,
							item.TotalCost,
							diff,
							item.CPUCost,
							item.RAMCost,
							item.GPUCost,
							//item.PVCost,
							item.NetworkCost,
							item.LoadBalancerCost,
							item.SharedCost,
							//item.ExternalCost,
						)
					}
				}
			}
		})
	}
}

func allocationComponentCost(item api.AllocationResponseItem) float64 {
	return item.CPUCost +
		item.RAMCost +
		item.GPUCost +
		//item.PVCost +
		item.NetworkCost +
		item.LoadBalancerCost +
		item.SharedCost
		//item.ExternalCost
	}


func withinCostTolerance(expected, actual float64) bool {
	diff := math.Abs(expected - actual)
	if diff <= allocationCostAbsoluteEpsilon {
		return true
	}

	larger := math.Max(math.Abs(expected), math.Abs(actual))
	if larger == 0 {
		return true
	}

	return diff/larger <= allocationCostRelativeEpsilon
}