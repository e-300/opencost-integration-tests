setup() {
    DIR="$( cd "$( dirname "$BATS_TEST_FILENAME" )" >/dev/null 2>&1 && pwd )"
    cd $DIR
}

teardown() {
    : # nothing to tear down
}

@test "schema: Allocation Response" {
    go test schema_stability_test.go -run TestAllocationResponseSchemaStability
}

@test "schema: Assets Response" {
    go test schema_stability_test.go -run TestAssetsResponseSchemaStability
}

@test "schema: CloudCost Response" {
    go test schema_stability_test.go -run TestCloudCostResponseSchemaStability
}