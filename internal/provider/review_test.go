package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/trunk-io/terraform-provider-trunk/internal/client"
	"github.com/trunk-io/terraform-provider-trunk/internal/fakeapi"
)

// untaint does what `terraform untaint` does, on the state files the test harness keeps under dir.
func untaint(t *testing.T, dir string) {
	t.Helper()
	found := false
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "terraform.tfstate" {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !strings.Contains(string(raw), `"status": "tainted"`) {
			return nil
		}
		found = true
		return os.WriteFile(path, []byte(strings.ReplaceAll(string(raw), `"status": "tainted",`, "")), 0o600)
	})
	if err != nil || !found {
		t.Fatalf("untaint: found=%v err=%v", found, err)
	}
}

// After a create whose follow-up configuration fails, `terraform untaint` and apply finish setting
// up the same queue in place instead of replacing it.
func TestMergeQueue_partialCreateUntaintFinishesInPlace(t *testing.T) {
	eachCredential(t, func(t *testing.T, server *fakeapi.Server, cred credential) {
		dir := t.TempDir()
		config := providerBlock(server, cred.key) + queueConfig(`merge_method = "rebase"`)
		var queueID string
		resource.UnitTest(t, resource.TestCase{
			ProtoV6ProviderFactories: factories,
			WorkingDir:               dir,
			Steps: []resource.TestStep{
				{
					PreConfig: func() {
						// The configuring request fails before reaching the service, on every retry.
						if cred.name == "v1" {
							// The first updateQueue is the check that the branch is free; it 404s.
							server.PassNext(http.MethodPost, "/v1/updateQueue")
							server.FailBefore(http.MethodPost, "/v1/updateQueue", 4)
						} else {
							server.FailBefore(http.MethodPatch, "/v2/merge-queues/mq1/config", 4)
						}
					},
					Config:      config,
					ExpectError: regexp.MustCompile(`terraform untaint`),
				},
				{
					PreConfig: func() {
						untaint(t, dir)
						queueID = server.Queue(repo, "main").ID
					},
					Config: config,
					ConfigPlanChecks: resource.ConfigPlanChecks{
						PreApply: []plancheck.PlanCheck{
							plancheck.ExpectResourceAction("trunk_merge_queue.main", plancheck.ResourceActionUpdate),
						},
					},
					Check: func(*terraform.State) error {
						q := server.Queue(repo, "main")
						if q.ID != queueID || q.MergeMethod != "rebase" {
							return fmt.Errorf("queue %s with %s, want %s with rebase", q.ID, q.MergeMethod, queueID)
						}
						return nil
					},
				},
			},
		})
	})
}

// Without a refresh, the plan carries the old queue ID; the update must refuse rather than write
// to the queue now on the branch.
func TestMergeQueue_replacedQueueRefusedWithoutRefresh(t *testing.T) {
	server := fakeapi.New(repo)
	defer server.Close()
	p := providerBlock(server, fakeapi.V2Key)
	var newID string
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		AdditionalCLIOptions:     &resource.AdditionalCLIOptions{Plan: resource.PlanOptions{NoRefresh: true}},
		Steps: []resource.TestStep{
			{Config: p + queueConfig(`concurrency = 6`)},
			{
				PreConfig:   func() { newID = server.Recreate(repo, "main") },
				Config:      p + queueConfig(`concurrency = 7`),
				ExpectError: regexp.MustCompile(`replaced outside Terraform(.|\n)*import the new queue`),
			},
		},
		CheckDestroy: func(*terraform.State) error {
			for _, r := range server.Requests() {
				if r.Method != http.MethodGet && strings.Contains(r.Path, newID) {
					return fmt.Errorf("touched the replacement queue: %s %s", r.Method, r.Path)
				}
			}
			if q := server.Queue(repo, "main"); q == nil || q.ID != newID || q.Concurrency != 5 {
				return fmt.Errorf("replacement queue = %+v", q)
			}
			return nil
		},
	})
}

// With a normal refresh, a recreated queue is not adopted: the resource leaves state, and the
// replacement on the branch is left alone (matching create, which refuses to take one over).
func TestMergeQueue_replacedQueueDroppedOnRefresh(t *testing.T) {
	server := fakeapi.New(repo)
	defer server.Close()
	p := providerBlock(server, fakeapi.V2Key)
	var newID string
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{Config: p + queueConfig(`concurrency = 6`)},
			{
				PreConfig: func() { newID = server.Recreate(repo, "main") },
				Config:    p + queueConfig(`concurrency = 7`),
				// The old ID is gone, so refresh drops the resource; apply then tries to create
				// and hits the replacement still on the branch.
				ExpectError: regexp.MustCompile(`already exists`),
			},
		},
		CheckDestroy: func(*terraform.State) error {
			for _, r := range server.Requests() {
				if r.Method == http.MethodDelete && strings.Contains(r.Path, newID) {
					return fmt.Errorf("deleted the replacement queue: %s", r.Path)
				}
			}
			if q := server.Queue(repo, "main"); q == nil || q.ID != newID || q.Concurrency != 5 {
				return fmt.Errorf("replacement queue = %+v", q)
			}
			return nil
		},
	})
}

// A create whose outcome can't be determined says so, records nothing, and the next apply points
// at import rather than creating a second queue.
func TestMergeQueue_createOutcomeUnknown(t *testing.T) {
	server := fakeapi.New(repo)
	defer server.Close()
	config := providerBlock(server, fakeapi.V2Key) + queueConfig(`concurrency = 6`)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					server.PassNext(http.MethodGet, "/v2/merge-queues") // the check that the branch is free
					server.FailAfter(http.MethodPost, "/v2/merge-queues", http.StatusServiceUnavailable)
					for range 4 {
						server.FailAfter(http.MethodGet, "/v2/merge-queues", http.StatusServiceUnavailable)
					}
				},
				Config:      config,
				ExpectError: regexp.MustCompile(`Merge queue may have been created`),
			},
			{
				Config:      config,
				ExpectError: regexp.MustCompile(`terraform import <resource address> "github.com/acme/widgets/main"`),
			},
		},
	})
}

// A queue that was created but couldn't be read back isn't recorded, and the error says to import it.
func TestMergeQueue_createdNotRecorded(t *testing.T) {
	server := fakeapi.New(repo)
	defer server.Close()
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{{
			PreConfig: func() {
				for range 4 {
					server.FailAfter(http.MethodPatch, "/v2/merge-queues/mq1/config", http.StatusBadGateway)
					server.FailAfter(http.MethodGet, "/v2/merge-queues/mq1", http.StatusBadGateway)
				}
			},
			Config:      providerBlock(server, fakeapi.V2Key) + queueConfig(`merge_method = "rebase"`),
			ExpectError: regexp.MustCompile(`Merge queue created but not recorded(.|\n)*terraform import`),
		}},
	})
}

// Switching from an API key to the organization token with no other change plans nothing, with
// or without a refresh.
func TestMergeQueue_switchV2ToV1NoChange(t *testing.T) {
	for _, noRefresh := range []bool{false, true} {
		t.Run(fmt.Sprintf("noRefresh=%v", noRefresh), func(t *testing.T) {
			server := fakeapi.New(repo)
			defer server.Close()
			body := queueConfig(`concurrency = 6`)
			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: factories,
				AdditionalCLIOptions:     &resource.AdditionalCLIOptions{Plan: resource.PlanOptions{NoRefresh: noRefresh}},
				Steps: []resource.TestStep{
					{Config: providerBlock(server, fakeapi.V2Key) + body},
					{
						Config: providerBlock(server, fakeapi.V1Token) + body,
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
						},
					},
				},
			})
		})
	}
}

// A queue imported mid-switch keeps the state the API reports, and changes wait for the switch.
func TestMergeQueue_importWhileSwitching(t *testing.T) {
	server := fakeapi.New(repo)
	defer server.Close()
	p := providerBlock(server, fakeapi.V2Key)
	c := client.NewClient(fakeapi.V2Key, server.URL, "test")
	concurrency := 6
	created, err := c.CreateQueue(context.Background(), client.CreateParams{
		Repo: client.Repo{Host: "github.com", Owner: "acme", Name: "widgets"}, TargetBranch: "main",
		Settings: client.Settings{Concurrency: &concurrency},
	})
	if err != nil {
		t.Fatal(err)
	}
	server.Edit(repo, "main", func(q *fakeapi.Queue) { q.State = "switching_modes" })
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config:             p + queueConfig(`concurrency = 6`),
				ResourceName:       "trunk_merge_queue.main",
				ImportState:        true,
				ImportStateId:      created.ID,
				ImportStatePersist: true,
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if got := states[0].Attributes["state"]; got != "switching_modes" {
						return fmt.Errorf("imported state = %q", got)
					}
					return nil
				},
			},
			{
				Config:      p + queueConfig(`concurrency = 7`),
				ExpectError: regexp.MustCompile(`switching modes`),
			},
		},
	})
}

// Import-by-ID records Trunk's repo spelling; a config that differs only in capitals must not
// destroy the queue (v2 create matches names exactly and would then fail).
func TestMergeQueue_importByIDCaseOnlyConfig(t *testing.T) {
	server := fakeapi.New(repo)
	defer server.Close()
	p := providerBlock(server, fakeapi.V2Key)
	c := client.NewClient(fakeapi.V2Key, server.URL, "test")
	concurrency := 6
	created, err := c.CreateQueue(context.Background(), client.CreateParams{
		Repo: client.Repo{Host: "github.com", Owner: "acme", Name: "widgets"}, TargetBranch: "main",
		Settings: client.Settings{Concurrency: &concurrency},
	})
	if err != nil {
		t.Fatal(err)
	}
	queueID := created.ID
	cased := p + `
resource "trunk_merge_queue" "main" {
  repo = {
    host  = "github.com"
    owner = "Acme"
    name  = "widgets"
  }
  target_branch = "main"
  concurrency   = 6
}
`
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config:             cased,
				ResourceName:       "trunk_merge_queue.main",
				ImportState:        true,
				ImportStateId:      queueID,
				ImportStatePersist: true,
			},
			{
				Config: cased,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("trunk_merge_queue.main", plancheck.ResourceActionUpdate),
					},
				},
				Check: func(*terraform.State) error {
					if q := server.Queue(repo, "main"); q == nil || q.ID != queueID {
						return fmt.Errorf("queue was replaced: %+v", q)
					}
					return nil
				},
			},
		},
	})
}

// A refresh with the organization token for a non-github.com host must not write. State is kept
// (with a warning) so destroy can still use deleteQueue, which honors the request host.
func TestMergeQueue_gheOrgTokenRefreshDoesNotWrite(t *testing.T) {
	const ghe = "github.example.com/acme/widgets"
	server := fakeapi.New(ghe)
	defer server.Close()
	config := providerBlock(server, fakeapi.V1Token) + `
resource "trunk_merge_queue" "main" {
  repo = {
    host  = "github.example.com"
    owner = "acme"
    name  = "widgets"
  }
  target_branch = "main"
  concurrency   = 6
}
`
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: server.ResetRequests,
				Config:    config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: func(*terraform.State) error {
					for _, r := range server.Requests() {
						if r.Path == "/v1/updateQueue" {
							return fmt.Errorf("refresh confirmed with a write: %s", r.Path)
						}
					}
					return nil
				},
			},
		},
	})
}
