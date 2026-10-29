package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/trunk-io/terraform-provider-trunk/internal/fakeapi"
)

// State written by the last 1.x release must load in this version. required_statuses was a
// list there and is a set here, and queue_id, managed_state, batching_rules and
// test_branch_construction_mode are new.
func TestMergeQueue_upgradeFromV1x(t *testing.T) {
	server := fakeapi.New(repo)
	defer server.Close()
	config := `
provider "trunk" {
  api_key  = "` + fakeapi.V1Token + `"
  base_url = "` + server.URL + `/v1"
}
` + queueConfig(`
  mode              = "parallel"
  concurrency       = 4
  merge_method      = "rebase"
  required_statuses = ["build", "test"] # 1.x fails on any other order
`)
	resource.Test(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: map[string]resource.ExternalProvider{
					"trunk": {Source: "trunk-io/trunk", VersionConstraint: "1.1.0"},
				},
				Config: config,
			},
			{
				ProtoV6ProviderFactories: factories,
				Config:                   config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}
