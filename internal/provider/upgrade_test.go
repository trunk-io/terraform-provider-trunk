package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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

// The 2.0 behavior change: 1.x cleared a queue's required-status override on every apply when the
// configuration left `required_statuses` out. After upgrading, leaving it out leaves the override
// alone, and `[]` still clears it.
func TestMergeQueue_upgradeLeavesOmittedRequiredStatuses(t *testing.T) {
	server := fakeapi.New(repo)
	defer server.Close()
	provider := `
provider "trunk" {
  api_key  = "` + fakeapi.V1Token + `"
  base_url = "` + server.URL + `/v1"
}
`
	omitted := provider + queueConfig(`concurrency = 4`)
	override := func() []string { return server.Queue(repo, "main").RequiredStatuses }
	resource.Test(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: map[string]resource.ExternalProvider{
					"trunk": {Source: "trunk-io/trunk", VersionConstraint: "1.1.0"},
				},
				Config: omitted,
			},
			{
				// Set in the Trunk app after the last 1.x apply.
				PreConfig: func() {
					server.Edit(repo, "main", func(q *fakeapi.Queue) { q.RequiredStatuses = []string{"ci"} })
				},
				ProtoV6ProviderFactories: factories,
				Config:                   omitted,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: func(*terraform.State) error {
					if got := override(); len(got) != 1 || got[0] != "ci" {
						return fmt.Errorf("override = %v, want [ci]", got)
					}
					return nil
				},
			},
			{
				ProtoV6ProviderFactories: factories,
				Config:                   provider + queueConfig(`concurrency = 4`+"\n"+`required_statuses = []`),
				Check: func(*terraform.State) error {
					if got := override(); len(got) != 0 {
						return fmt.Errorf("override = %v, want none", got)
					}
					return nil
				},
			},
		},
	})
}
