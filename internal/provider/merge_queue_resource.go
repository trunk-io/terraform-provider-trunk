package provider

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/trunk-io/terraform-provider-trunk/internal/client"
)

var (
	_ resource.Resource                = &mergeQueueResource{}
	_ resource.ResourceWithConfigure   = &mergeQueueResource{}
	_ resource.ResourceWithImportState = &mergeQueueResource{}
	_ resource.ResourceWithModifyPlan  = &mergeQueueResource{}
)

type mergeQueueResource struct {
	client *client.Client
}

// NewMergeQueueResource returns the trunk_merge_queue resource.
func NewMergeQueueResource() resource.Resource {
	return &mergeQueueResource{}
}

func (r *mergeQueueResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_merge_queue"
}

// Every setting is Optional and Computed: an attribute left out of the configuration is not
// managed, and keeps whatever value the queue has.

func optionalString(description string, validators ...validator.String) schema.StringAttribute {
	return schema.StringAttribute{
		Description:   description,
		Optional:      true,
		Computed:      true,
		Validators:    validators,
		PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
	}
}

func optionalInt(description string, validators ...validator.Int64) schema.Int64Attribute {
	return schema.Int64Attribute{
		Description:   description,
		Optional:      true,
		Computed:      true,
		Validators:    validators,
		PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
	}
}

func optionalBool(description string) schema.BoolAttribute {
	return schema.BoolAttribute{
		Description:   description,
		Optional:      true,
		Computed:      true,
		PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
	}
}

func identity(description string) schema.StringAttribute {
	return schema.StringAttribute{
		Description:   description,
		Required:      true,
		Validators:    []validator.String{stringvalidator.LengthAtLeast(1)},
		PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
	}
}

// A batching rule pattern matches exactly, or as a prefix when it ends in one `*`.
var targetPatternRegexp = regexp.MustCompile(`^[^*\s](?:[^*]*[^*\s])?\*?$`)

func (r *mergeQueueResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Trunk merge queue: one per repository and target branch.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The queue's address, `{host}/{owner}/{name}/{target_branch}`.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"queue_id": schema.StringAttribute{
				Description:   "The queue's ID in the v2 API. Null when the provider uses an organization API token (v1), which has no queue IDs.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"managed_state": schema.StringAttribute{
				Description: "Whether the queue is managed by Terraform: `manual`, `terraform`, or `terraform_with_drift` " +
					"(changed outside Terraform since the last apply). Only the v2 API reports it; null with an organization API token.",
				Computed: true,
			},
			"repo": schema.SingleNestedAttribute{
				Description: "The repository the queue belongs to. Changing it replaces the queue.",
				Required:    true,
				Attributes: map[string]schema.Attribute{
					"host":  identity("Repository host, such as `github.com`."),
					"owner": identity("Repository owner (organization or user)."),
					"name":  identity("Repository name."),
				},
			},
			"target_branch": identity("The branch the queue merges into. Changing it replaces the queue."),

			"mode": optionalString(
				"`single` tests pull requests one after another; `parallel` tests independent pull requests in parallel, using impacted targets. "+
					"Changing it on an existing queue starts a mode switch, which finishes once the queue is empty.",
				stringvalidator.OneOf(client.ModeSingle, client.ModeParallel)),
			"state": optionalString(
				"`running`, `paused` (stops testing and merging), or `draining` (finishes queued pull requests and accepts no new ones).",
				stringvalidator.OneOf(client.StateRunning, client.StatePaused, client.StateDraining)),
			"concurrency": optionalInt("How many pull requests (or batches) the queue tests at once. Up to 1000 with an API key.",
				int64validator.AtLeast(1)),
			"bisection_concurrency": optionalInt(
				"How many tests may run at once while bisecting a failed batch. `0` shares `concurrency` instead of reserving slots; "+
					"`0` needs an API key, since the v1 API requires at least 1.",
				int64validator.AtLeast(0)),
			"testing_timeout_minutes": optionalInt("Minutes to wait for a pull request's tests before failing it. `0` means no timeout.",
				int64validator.AtLeast(0)),
			"pending_failure_depth": optionalInt(
				"How many levels of later test runs to wait for before removing a failed pull request. With `can_optimistically_merge`, a later run that passes clears a failure caused by a flaky test. `0` doesn't wait.",
				int64validator.AtLeast(0)),
			"can_optimistically_merge": optionalBool("When a pull request passes before those ahead of it finish testing, merge it and them together, since its tests included their changes."),
			"batch":                    optionalBool("Test several pull requests together as one batch."),
			"batching_max_wait_time_minutes": optionalInt(
				"Minutes to wait for a batch to reach `batching_min_size` before testing it anyway. `0` never waits; `0` needs an API key, since the v1 API requires at least 1.",
				int64validator.AtLeast(0)),
			"batching_min_size": optionalInt("How many pull requests a batch waits for.", int64validator.AtLeast(1)),
			"batching_rules": schema.SetNestedAttribute{
				Description: "Rules that change how pull requests are batched based on the build targets they impact. " +
					"Only applies in `parallel` mode with impacted targets uploaded. Set to `[]` to remove every rule.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"target_pattern": schema.StringAttribute{
							Description: "An impacted target, matched exactly, or as a prefix when it ends in `*` (for example `//db/migrations:*`). " +
								"`*` anywhere else, or on its own, is not allowed.",
							Required: true,
							Validators: []validator.String{
								stringvalidator.RegexMatches(targetPatternRegexp,
									"must be a target, or a prefix ending in a single *, with no leading or trailing spaces"),
							},
						},
						"policy": schema.StringAttribute{
							Description: "What to do with a matching pull request. `isolate` tests it on its own, as `/trunk merge --no-batch` does.",
							Required:    true,
							Validators:  []validator.String{stringvalidator.OneOf("isolate")},
						},
					},
				},
			},
			"create_prs_for_testing_branches": optionalBool(
				"Create a draft pull request for each test branch, so pull-request CI runs on it. When false, CI must run on pushes to `trunk-merge/` branches."),
			"merge_method": optionalString("How pull requests are merged: `merge_commit`, `squash`, or `rebase`.",
				stringvalidator.OneOf("merge_commit", "squash", "rebase")),
			"comments_enabled":       optionalBool("Post comments on pull requests about their progress through the queue."),
			"commands_enabled":       optionalBool("Accept `/trunk` commands in pull request comments."),
			"status_check_enabled":   optionalBool("Post a GitHub check on queued pull requests."),
			"extension_enabled":      optionalBool("Show the Trunk browser extension's merge queue controls for this repository."),
			"label_commands_enabled": optionalBool("Let a GitHub label enqueue a pull request (see `enqueueing_label`)."),
			"state_labels_enabled":   optionalBool("Label pull requests with their merge queue state."),
			"enqueueing_label": optionalString("The GitHub label that enqueues a pull request when `label_commands_enabled` is true.",
				stringvalidator.LengthAtLeast(1)),
			"direct_merge_mode": optionalString("`always` merges a pull request right away when it is already based on the tip of the target branch, the queue is empty and its tests passed; `off` always tests it in the queue.",
				stringvalidator.OneOf("off", "always")),
			"optimization_mode": optionalString("`bisection_skip_redundant_tests`: while bisecting a failed batch, don't test again a set of pull requests that already failed in the same bisection. `off` tests every set.",
				stringvalidator.OneOf("off", "bisection_skip_redundant_tests")),
			"test_branch_construction_mode": optionalString(
				"How the queue creates the `trunk-merge/` branch it tests on. `rename_temp_branch` builds it on a `trunk-temp/` branch and renames it. "+
					"`push_new_branch_from_temp` creates it directly; use it when a GitHub ruleset on `trunk-temp/` branches blocks renaming them.",
				stringvalidator.OneOf("rename_temp_branch", "push_new_branch_from_temp")),
			"not_ready_timeout_hours": optionalInt(
				"Hours a submitted pull request may stay not ready to merge before it is removed. `0` means no timeout. Up to 720 with an API key.",
				int64validator.AtLeast(0)),
			"required_statuses": schema.SetAttribute{
				Description: "The checks a pull request's tests must pass, overriding `.trunk/trunk.yaml` and branch protection. " +
					"Set to `[]` to remove the override and use those again; an override can't be empty. Leave it out to leave the queue's override as it is.",
				ElementType:   types.StringType,
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

// ModifyPlan plans managed_state. An apply that changes the queue writes as Terraform, which
// marks it Terraform-managed (clearing any drift); a plan with no other change leaves it alone.
// Only the v2 API reports it, so it stays null with an organization API token.
func (r *mergeQueueResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	var prior types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("managed_state"), &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	planned := prior
	if !onlyManagedStateDiffers(req.State.Raw, req.Plan.Raw) && !prior.IsNull() {
		planned = types.StringValue(client.ManagedStateTerraform)
	}
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("managed_state"), planned)...)
}

// onlyManagedStateDiffers reports whether plan and state match apart from managed_state.
func onlyManagedStateDiffers(state, plan tftypes.Value) bool {
	var s, p map[string]tftypes.Value
	if state.As(&s) != nil || plan.As(&p) != nil {
		return false
	}
	for k, v := range s {
		if k != "managed_state" && !v.Equal(p[k]) {
			return false
		}
	}
	return true
}

func (r *mergeQueueResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected resource configure type",
			fmt.Sprintf("Expected *client.Client, got %T. Report this issue to the provider developers.", req.ProviderData))
		return
	}
	r.client = c
}

func describe(m *mergeQueueResourceModel) string {
	repo := m.repo()
	return fmt.Sprintf("%s/%s/%s branch %q", repo.Host, repo.Owner, repo.Name, m.TargetBranch.ValueString())
}

func importID(m *mergeQueueResourceModel) string {
	repo := m.repo()
	return repo.Host + "/" + repo.Owner + "/" + repo.Name + "/" + m.TargetBranch.ValueString()
}

func (r *mergeQueueResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, config mergeQueueResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	params, diags := plan.createParams(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	queue, err := r.client.CreateQueue(ctx, params)
	if errors.Is(err, client.ErrAlreadyExists) {
		resp.Diagnostics.AddError("Merge queue already exists",
			fmt.Sprintf("A merge queue already exists for %s. Import it with:\n\n  terraform import <resource address> %q",
				describe(&plan), importID(&plan)))
		return
	}
	if queue != nil {
		// Record the queue even when configuring it failed, so the next apply finishes the job
		// instead of trying to create it again.
		state := plan
		_, d := state.fromQueue(queue, &plan)
		resp.Diagnostics.Append(d...)
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	}
	if err != nil {
		resp.Diagnostics.AddError("Error creating merge queue", fmt.Sprintf("Could not create the merge queue for %s: %s", describe(&plan), err))
	}
}

func (r *mergeQueueResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state mergeQueueResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	queue, err := r.client.GetQueue(ctx, state.ref())
	if errors.Is(err, client.ErrNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading merge queue", fmt.Sprintf("Could not read the merge queue for %s: %s", describe(&state), err))
		return
	}

	prior := state
	switching, diags := state.fromQueue(queue, &prior)
	resp.Diagnostics.Append(diags...)
	if switching {
		resp.Diagnostics.AddWarning("Merge queue is switching modes",
			fmt.Sprintf("The merge queue for %s is switching to %s mode, which finishes once the queue is empty. "+
				"Until then Terraform shows the mode it is switching to, and changes to the queue are refused.",
				describe(&state), state.Mode.ValueString()))
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *mergeQueueResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, config, prior mergeQueueResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	params, diags := plan.updateParams(ctx, &config, &prior)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	queue, err := r.client.UpdateQueue(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError("Error updating merge queue", fmt.Sprintf("Could not update the merge queue for %s: %s", describe(&plan), err))
		return
	}
	state := plan
	_, d := state.fromQueue(queue, &plan)
	resp.Diagnostics.Append(d...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *mergeQueueResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state mergeQueueResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.DeleteQueue(ctx, state.ref())
	if errors.Is(err, client.ErrQueueNotEmpty) {
		resp.Diagnostics.AddError("Merge queue is not empty",
			fmt.Sprintf("The merge queue for %s still has pull requests. Set state = \"draining\", apply, "+
				"and destroy again once the queue is empty.", describe(&state)))
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error deleting merge queue", fmt.Sprintf("Could not delete the merge queue for %s: %s", describe(&state), err))
	}
}

// ImportState accepts `{host}/{owner}/{name}/{target_branch}`, or a v2 queue ID with an API key.
func (r *mergeQueueResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := strings.TrimSpace(req.ID)
	if !strings.Contains(id, "/") {
		if r.client != nil && r.client.Version() != client.V2 {
			resp.Diagnostics.AddError("Invalid import ID",
				fmt.Sprintf("%q looks like a v2 queue ID, which needs an API key. With an organization API token, import by "+
					"{host}/{owner}/{name}/{target_branch}.", id))
			return
		}
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("queue_id"), id)...)
		return
	}
	// SplitN keeps slashes in branch names, such as release/1.0.
	parts := strings.SplitN(id, "/", 4)
	if len(parts) != 4 || parts[0] == "" || parts[1] == "" || parts[2] == "" || parts[3] == "" {
		resp.Diagnostics.AddError("Invalid import ID",
			fmt.Sprintf("Expected {host}/{owner}/{name}/{target_branch}, or a v2 queue ID; got %q.", id))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("repo"), repoModel{
		Host:  types.StringValue(parts[0]),
		Owner: types.StringValue(parts[1]),
		Name:  types.StringValue(parts[2]),
	})...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("target_branch"), parts[3])...)
}
