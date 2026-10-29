package provider

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/trunk-io/terraform-provider-trunk/internal/fakeapi"
)

// These tests run Terraform against fakeapi, once with the organization API token (v1) and once
// with an API key (v2). They need a terraform binary on PATH but no Trunk account.

const repo = "github.com/acme/widgets"

var factories = map[string]func() (tfprotov6.ProviderServer, error){
	"trunk": providerserver.NewProtocol6WithError(New("test")()),
}

type credential struct {
	name string
	key  string
}

var credentials = []credential{
	{"v1", fakeapi.V1Token},
	{"v2", fakeapi.V2Key},
}

func providerBlock(server *fakeapi.Server, key string) string {
	return fmt.Sprintf(`
provider "trunk" {
  api_key  = %q
  base_url = %q
}
`, key, server.URL)
}

func eachCredential(t *testing.T, fn func(t *testing.T, server *fakeapi.Server, cred credential)) {
	for _, cred := range credentials {
		t.Run(cred.name, func(t *testing.T) {
			server := fakeapi.New(repo)
			defer server.Close()
			fn(t, server, cred)
		})
	}
}

func queueConfig(body string) string {
	return `
resource "trunk_merge_queue" "main" {
  repo = {
    host  = "github.com"
    owner = "acme"
    name  = "widgets"
  }
  target_branch = "main"
` + body + `
}
`
}

func TestMergeQueue_lifecycle(t *testing.T) {
	eachCredential(t, func(t *testing.T, server *fakeapi.Server, cred credential) {
		p := providerBlock(server, cred.key)
		resource.UnitTest(t, resource.TestCase{
			ProtoV6ProviderFactories: factories,
			Steps: []resource.TestStep{
				{
					Config: p + queueConfig(`
  mode                          = "parallel"
  concurrency                   = 10
  bisection_concurrency         = 3
  testing_timeout_minutes       = 0
  pending_failure_depth         = 2
  can_optimistically_merge      = true
  batch                         = true
  batching_max_wait_time_minutes = 10
  batching_min_size             = 4
  merge_method                  = "merge_commit"
  comments_enabled              = false
  commands_enabled              = false
  status_check_enabled          = false
  extension_enabled             = false
  label_commands_enabled        = false
  state_labels_enabled          = true
  enqueueing_label              = "ship-it"
  direct_merge_mode             = "always"
  optimization_mode             = "bisection_skip_redundant_tests"
  test_branch_construction_mode = "push_new_branch_from_temp"
  not_ready_timeout_hours       = 48
  required_statuses             = ["test", "build"]
  batching_rules = [
    { target_pattern = "//db/migrations:*", policy = "isolate" },
  ]
`),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("trunk_merge_queue.main", "id", repo+"/main"),
						resource.TestCheckResourceAttr("trunk_merge_queue.main", "mode", "parallel"),
						resource.TestCheckResourceAttr("trunk_merge_queue.main", "state", "running"),
						resource.TestCheckResourceAttr("trunk_merge_queue.main", "concurrency", "10"),
						resource.TestCheckResourceAttr("trunk_merge_queue.main", "merge_method", "merge_commit"),
						resource.TestCheckResourceAttr("trunk_merge_queue.main", "optimization_mode", "bisection_skip_redundant_tests"),
						resource.TestCheckResourceAttr("trunk_merge_queue.main", "test_branch_construction_mode", "push_new_branch_from_temp"),
						resource.TestCheckResourceAttr("trunk_merge_queue.main", "not_ready_timeout_hours", "48"),
						resource.TestCheckResourceAttr("trunk_merge_queue.main", "required_statuses.#", "2"),
						resource.TestCheckResourceAttr("trunk_merge_queue.main", "batching_rules.#", "1"),
						resource.TestCheckResourceAttr("trunk_merge_queue.main", "batching_rules.0.target_pattern", "//db/migrations:*"),
						func(_ *terraform.State) error {
							q := server.Queue(repo, "main")
							if q.Mode != "parallel" || q.EnqueueingLabel != "ship-it" || q.NotReadyTimeoutHours != 48 ||
								fmt.Sprint(q.RequiredStatuses) != "[build test]" || q.ManagedState != "terraform" {
								return fmt.Errorf("server queue = %+v", q)
							}
							return nil
						},
						checkV2Attrs(cred),
					),
				},
				// Same config again: nothing to change, whatever order the API returns things in.
				{
					Config:   p + queueConfig(`mode = "parallel"`+"\n"+`required_statuses = ["test", "build"]`),
					PlanOnly: true,
				},
				{
					Config: p + queueConfig(`
  state       = "paused"
  concurrency = 3
`),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("trunk_merge_queue.main", "state", "paused"),
						resource.TestCheckResourceAttr("trunk_merge_queue.main", "concurrency", "3"),
						// Left out of the config, so left alone.
						resource.TestCheckResourceAttr("trunk_merge_queue.main", "required_statuses.#", "2"),
						resource.TestCheckResourceAttr("trunk_merge_queue.main", "enqueueing_label", "ship-it"),
					),
				},
				{
					ResourceName:      "trunk_merge_queue.main",
					ImportState:       true,
					ImportStateId:     repo + "/main",
					ImportStateVerify: true,
				},
			},
		})
	})
}

func checkV2Attrs(cred credential) resource.TestCheckFunc {
	if cred.name == "v1" {
		return resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckNoResourceAttr("trunk_merge_queue.main", "queue_id"),
			resource.TestCheckNoResourceAttr("trunk_merge_queue.main", "managed_state"),
		)
	}
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet("trunk_merge_queue.main", "queue_id"),
		resource.TestCheckResourceAttr("trunk_merge_queue.main", "managed_state", "terraform"),
	)
}

// Leaving required_statuses out must not touch an override set elsewhere; [] removes it.
func TestMergeQueue_requiredStatuses(t *testing.T) {
	eachCredential(t, func(t *testing.T, server *fakeapi.Server, cred credential) {
		p := providerBlock(server, cred.key)
		resource.UnitTest(t, resource.TestCase{
			ProtoV6ProviderFactories: factories,
			Steps: []resource.TestStep{
				{Config: p + queueConfig(`concurrency = 2`)},
				{
					PreConfig: func() {
						server.Edit(repo, "main", func(q *fakeapi.Queue) { q.RequiredStatuses = []string{"ci"} })
					},
					Config: p + queueConfig(`concurrency = 4`),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("trunk_merge_queue.main", "required_statuses.#", "1"),
						func(_ *terraform.State) error {
							if got := server.Queue(repo, "main").RequiredStatuses; fmt.Sprint(got) != "[ci]" {
								return fmt.Errorf("override = %v, want [ci]", got)
							}
							return nil
						},
					),
				},
				{
					Config: p + queueConfig(`required_statuses = []`),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("trunk_merge_queue.main", "required_statuses.#", "0"),
						func(_ *terraform.State) error {
							if got := server.Queue(repo, "main").RequiredStatuses; len(got) != 0 {
								return fmt.Errorf("override = %v, want none", got)
							}
							return nil
						},
					),
				},
			},
		})
	})
}

// A change made outside Terraform shows up as a diff, and on v2 as drift.
func TestMergeQueue_drift(t *testing.T) {
	eachCredential(t, func(t *testing.T, server *fakeapi.Server, cred credential) {
		p := providerBlock(server, cred.key)
		config := p + queueConfig(`merge_method = "rebase"`)
		resource.UnitTest(t, resource.TestCase{
			ProtoV6ProviderFactories: factories,
			Steps: []resource.TestStep{
				{Config: config},
				{
					PreConfig: func() {
						server.Edit(repo, "main", func(q *fakeapi.Queue) { q.MergeMethod = "squash" })
					},
					Config: config,
					ConfigPlanChecks: resource.ConfigPlanChecks{
						PreApply: []plancheck.PlanCheck{
							plancheck.ExpectResourceAction("trunk_merge_queue.main", plancheck.ResourceActionUpdate),
						},
					},
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("trunk_merge_queue.main", "merge_method", "rebase"),
						checkV2Attrs(cred),
					),
				},
			},
		})
	})
}

// Changing the mode starts a switch. Until it finishes, Terraform keeps the target mode and
// refuses further changes rather than abandoning the switch.
func TestMergeQueue_modeSwitch(t *testing.T) {
	eachCredential(t, func(t *testing.T, server *fakeapi.Server, cred credential) {
		p := providerBlock(server, cred.key)
		resource.UnitTest(t, resource.TestCase{
			ProtoV6ProviderFactories: factories,
			Steps: []resource.TestStep{
				{Config: p + queueConfig(`mode = "single"`+"\n"+`state = "running"`)},
				{
					Config: p + queueConfig(`mode = "parallel"`+"\n"+`state = "running"`),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("trunk_merge_queue.main", "mode", "parallel"),
						resource.TestCheckResourceAttr("trunk_merge_queue.main", "state", "running"),
						func(_ *terraform.State) error {
							if q := server.Queue(repo, "main"); q.State != "switching_modes" {
								return fmt.Errorf("server state = %s, want switching_modes", q.State)
							}
							return nil
						},
					),
				},
				{
					Config:      p + queueConfig(`mode = "parallel"`+"\n"+`state = "running"`+"\n"+`concurrency = 9`),
					ExpectError: regexp.MustCompile(`switching modes`),
				},
				{
					PreConfig: server.FinishSwitches,
					Config:    p + queueConfig(`mode = "parallel"`+"\n"+`state = "running"`+"\n"+`concurrency = 9`),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("trunk_merge_queue.main", "mode", "parallel"),
						resource.TestCheckResourceAttr("trunk_merge_queue.main", "concurrency", "9"),
					),
				},
			},
		})
	})
}

func TestMergeQueue_errors(t *testing.T) {
	eachCredential(t, func(t *testing.T, server *fakeapi.Server, cred credential) {
		p := providerBlock(server, cred.key)
		resource.UnitTest(t, resource.TestCase{
			ProtoV6ProviderFactories: factories,
			Steps: []resource.TestStep{
				{Config: p + queueConfig(``)},
				// A second resource on the same branch is told to import instead.
				{
					Config: p + queueConfig(``) + `
resource "trunk_merge_queue" "dup" {
  repo = {
    host  = "github.com"
    owner = "acme"
    name  = "widgets"
  }
  target_branch = "main"
}
`,
					ExpectError: regexp.MustCompile(`terraform import <resource address> "github.com/acme/widgets/main"`),
				},
				{
					PreConfig: func() { server.Edit(repo, "main", func(q *fakeapi.Queue) { q.Entries = 1 }) },
					Config:    p,
					// Destroying a queue that still has pull requests explains what to do.
					ExpectError: regexp.MustCompile(`Set state = "draining"`),
				},
				{
					PreConfig: func() { server.Edit(repo, "main", func(q *fakeapi.Queue) { q.Entries = 0 }) },
					Config:    p,
				},
			},
		})
	})
}

func TestMergeQueue_importByQueueID(t *testing.T) {
	server := fakeapi.New(repo)
	defer server.Close()
	p := providerBlock(server, fakeapi.V2Key)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{Config: p + queueConfig(`concurrency = 7`)},
			{
				ResourceName:      "trunk_merge_queue.main",
				ImportState:       true,
				ImportStateIdFunc: func(*terraform.State) (string, error) { return server.Queue(repo, "main").ID, nil },
				ImportStateVerify: true,
			},
		},
	})
}

func TestMergeQueue_validation(t *testing.T) {
	server := fakeapi.New(repo)
	defer server.Close()
	p := providerBlock(server, fakeapi.V1Token)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config:      p + queueConfig(`merge_method = "SQUASH"`),
				ExpectError: regexp.MustCompile(`merge_method`),
			},
			{
				Config:      p + queueConfig(`batching_rules = [{ target_pattern = "*", policy = "isolate" }]`),
				ExpectError: regexp.MustCompile(`target_pattern`),
			},
			{
				Config:      p + queueConfig(`batching_rules = [{ target_pattern = "//a*b", policy = "isolate" }]`),
				ExpectError: regexp.MustCompile(`target_pattern`),
			},
		},
	})
}

// A create that fails after the queue exists records it, so the next apply doesn't hit
// "already exists". For v2 the failure is in the create request itself, which v2 can answer with
// an error after committing the queue.
func TestMergeQueue_partialCreate(t *testing.T) {
	eachCredential(t, func(t *testing.T, server *fakeapi.Server, cred credential) {
		p := providerBlock(server, cred.key)
		config := p + queueConfig(`concurrency = 6`)
		resource.UnitTest(t, resource.TestCase{
			ProtoV6ProviderFactories: factories,
			Steps: []resource.TestStep{
				{
					PreConfig: func() {
						if cred.name == "v1" {
							// updateQueue is retried on 5xx; fail every attempt.
							for range 4 {
								server.FailAfter(http.MethodPost, "/v1/updateQueue", http.StatusInternalServerError)
							}
						} else {
							server.FailAfter(http.MethodPost, "/v2/merge-queues", http.StatusInternalServerError)
						}
					},
					Config:      config,
					ExpectError: regexp.MustCompile(`created but not fully configured`),
				},
				{
					Config: config,
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("trunk_merge_queue.main", "concurrency", "6"),
					),
				},
			},
		})
	})
}

// A refused token must not make Terraform forget a queue. v1's getQueue answers a bad token with
// 404, the same as a deleted queue.
func TestMergeQueue_badTokenKeepsState(t *testing.T) {
	eachCredential(t, func(t *testing.T, server *fakeapi.Server, cred credential) {
		good := providerBlock(server, cred.key)
		bad := providerBlock(server, cred.key[:len(cred.key)-1]+"x")
		resource.UnitTest(t, resource.TestCase{
			ProtoV6ProviderFactories: factories,
			Steps: []resource.TestStep{
				{Config: good + queueConfig(`concurrency = 6`)},
				{
					Config:      bad + queueConfig(`concurrency = 6`),
					ExpectError: regexp.MustCompile(`401`),
				},
				{
					Config: good + queueConfig(`concurrency = 6`),
					ConfigPlanChecks: resource.ConfigPlanChecks{
						PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
					},
				},
			},
		})
	})
}

// v1 matches repository names ignoring case and v2 doesn't, so a configuration that worked with
// the organization token must keep working, unchanged, with an API key.
func TestMergeQueue_switchToAPIKeyWithDifferentCase(t *testing.T) {
	server := fakeapi.New(repo)
	defer server.Close()
	config := queueConfig(`concurrency = 6`)
	config = regexp.MustCompile(`owner = "acme"`).ReplaceAllString(config, `owner = "Acme"`)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{Config: providerBlock(server, fakeapi.V1Token) + config},
			{
				Config: providerBlock(server, fakeapi.V2Key) + config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("trunk_merge_queue.main", "repo.owner", "Acme"),
					resource.TestCheckResourceAttrSet("trunk_merge_queue.main", "queue_id"),
				),
			},
			{
				Config: providerBlock(server, fakeapi.V2Key) + regexp.MustCompile(`concurrency = 6`).ReplaceAllString(config, `concurrency = 7`),
				Check:  resource.TestCheckResourceAttr("trunk_merge_queue.main", "managed_state", "terraform"),
			},
			// And back to the token: the v2-only attributes go back to null.
			{
				Config: providerBlock(server, fakeapi.V1Token) + regexp.MustCompile(`concurrency = 6`).ReplaceAllString(config, `concurrency = 8`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("trunk_merge_queue.main", "queue_id"),
					resource.TestCheckNoResourceAttr("trunk_merge_queue.main", "managed_state"),
				),
			},
		},
	})
}

func TestMergeQueue_bisectionConcurrencyAtLeastOne(t *testing.T) {
	server := fakeapi.New(repo)
	defer server.Close()
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{{
			Config:      providerBlock(server, fakeapi.V2Key) + queueConfig(`bisection_concurrency = 0`),
			ExpectError: regexp.MustCompile(`bisection_concurrency`),
		}},
	})
}
