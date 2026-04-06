package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/trunk-io/terraform-provider-trunk/internal/client"
)

// Compile-time interface assertions.
var (
	_ resource.Resource                = &mergeQueueResource{}
	_ resource.ResourceWithConfigure   = &mergeQueueResource{}
	_ resource.ResourceWithImportState = &mergeQueueResource{}
)

type mergeQueueResource struct {
	client *client.Client
}

// NewMergeQueueResource returns a new mergeQueueResource.
func NewMergeQueueResource() resource.Resource {
	return &mergeQueueResource{}
}

func (r *mergeQueueResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_merge_queue"
}

func (r *mergeQueueResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Trunk merge queue.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Unique identifier for this queue in the format {host}/{owner}/{name}/{target_branch}.",
				Computed:    true,
			},
			"repo": schema.SingleNestedAttribute{
				Description: "Repository this queue is associated with.",
				Required:    true,
				Attributes: map[string]schema.Attribute{
					"host": schema.StringAttribute{
						Description: "Repository host (e.g., \"github.com\").",
						Required:    true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"owner": schema.StringAttribute{
						Description: "Repository owner (organization or user).",
						Required:    true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"name": schema.StringAttribute{
						Description: "Repository name.",
						Required:    true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
				},
			},
			"target_branch": schema.StringAttribute{
				Description: "Target branch for the merge queue.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"mode": schema.StringAttribute{
				Description: "Queue mode: \"single\" or \"parallel\".",
				Optional:    true,
				Computed:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("single", "parallel"),
				},
			},
			"concurrency": schema.Int64Attribute{
				Description: "Number of concurrent test slots.",
				Optional:    true,
				Computed:    true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"state": schema.StringAttribute{
				Description: "Queue state: \"RUNNING\", \"PAUSED\", or \"DRAINING\".",
				Optional:    true,
				Computed:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("RUNNING", "PAUSED", "DRAINING"),
				},
			},
			"testing_timeout_minutes": schema.Int64Attribute{
				Description: "Maximum minutes to wait for tests.",
				Optional:    true,
				Computed:    true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"pending_failure_depth": schema.Int64Attribute{
				Description: "Number of PRs below a failure to wait for before eviction.",
				Optional:    true,
				Computed:    true,
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
				},
			},
			"can_optimistically_merge": schema.BoolAttribute{
				Description: "Allow optimistic merge when a lower PR passes.",
				Optional:    true,
				Computed:    true,
			},
			"batch": schema.BoolAttribute{
				Description: "Enable batching.",
				Optional:    true,
				Computed:    true,
			},
			"batching_max_wait_time_minutes": schema.Int64Attribute{
				Description: "Maximum minutes to wait for a batch to fill.",
				Optional:    true,
				Computed:    true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"batching_min_size": schema.Int64Attribute{
				Description: "Minimum number of PRs per batch.",
				Optional:    true,
				Computed:    true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"merge_method": schema.StringAttribute{
				Description: "Merge method: \"MERGE_COMMIT\", \"SQUASH\", or \"REBASE\".",
				Optional:    true,
				Computed:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("MERGE_COMMIT", "SQUASH", "REBASE"),
				},
			},
			"comments_enabled": schema.BoolAttribute{
				Description: "Post GitHub comments on PRs.",
				Optional:    true,
				Computed:    true,
			},
			"commands_enabled": schema.BoolAttribute{
				Description: "Allow /trunk merge comments.",
				Optional:    true,
				Computed:    true,
			},
			"create_prs_for_testing_branches": schema.BoolAttribute{
				Description: "Create PRs for testing branches.",
				Optional:    true,
				Computed:    true,
			},
			"status_check_enabled": schema.BoolAttribute{
				Description: "Post GitHub status checks.",
				Optional:    true,
				Computed:    true,
			},
			"direct_merge_mode": schema.StringAttribute{
				Description: "Direct merge mode: \"OFF\" or \"ALWAYS\".",
				Optional:    true,
				Computed:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("OFF", "ALWAYS"),
				},
			},
			"optimization_mode": schema.StringAttribute{
				Description: "Optimization mode: \"OFF\" or \"BISECTION_SKIP_REDUNDANT_TESTS\".",
				Optional:    true,
				Computed:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("OFF", "BISECTION_SKIP_REDUNDANT_TESTS"),
				},
			},
			"bisection_concurrency": schema.Int64Attribute{
				Description: "Number of concurrent tests during bisection.",
				Optional:    true,
				Computed:    true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"required_statuses": schema.ListAttribute{
				Description: "Override required status checks. Set to null to revert to branch protection or trunk.yaml defaults; set to [] to explicitly require no statuses.",
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (r *mergeQueueResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = c
}

func (r *mergeQueueResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var model mergeQueueResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Step 1: create the queue with identity fields, mode, and concurrency.
	_, err := r.client.CreateQueue(ctx, model.toCreateRequest())
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 409 {
			resp.Diagnostics.AddError(
				"Merge queue already exists",
				fmt.Sprintf(
					"A merge queue for %s/%s/%s already exists on branch %q. "+
						"Import it into Terraform state with:\n\n"+
						"  terraform import trunk_merge_queue.<name> %q",
					model.Repo.Host.ValueString(),
					model.Repo.Owner.ValueString(),
					model.Repo.Name.ValueString(),
					model.TargetBranch.ValueString(),
					model.Repo.Host.ValueString()+"/"+
						model.Repo.Owner.ValueString()+"/"+
						model.Repo.Name.ValueString()+"/"+
						model.TargetBranch.ValueString(),
				),
			)
			return
		}
		resp.Diagnostics.AddError("Error creating merge queue", err.Error())
		return
	}

	// Step 2: apply all remaining optional attributes. If this fails, the queue exists but
	// is partially configured. Set partial state so a subsequent apply triggers Update.
	_, err = r.client.UpdateQueue(ctx, model.toUpdateRequest())
	if err != nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
		resp.Diagnostics.AddError("Error configuring merge queue after creation", err.Error())
		return
	}

	// Step 3: read back the full authoritative state from the API.
	queue, err := r.client.GetQueue(ctx, client.GetQueueRequest{
		Repo:         client.Repo{Host: model.Repo.Host.ValueString(), Owner: model.Repo.Owner.ValueString(), Name: model.Repo.Name.ValueString()},
		TargetBranch: model.TargetBranch.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error reading merge queue after creation", err.Error())
		return
	}

	model.fromQueue(queue)
	model.setID()
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func (r *mergeQueueResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model mergeQueueResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	queue, err := r.client.GetQueue(ctx, client.GetQueueRequest{
		Repo:         client.Repo{Host: model.Repo.Host.ValueString(), Owner: model.Repo.Owner.ValueString(), Name: model.Repo.Name.ValueString()},
		TargetBranch: model.TargetBranch.ValueString(),
	})
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading merge queue",
			fmt.Sprintf("Could not read merge queue %s/%s/%s branch %q: %s",
				model.Repo.Host.ValueString(), model.Repo.Owner.ValueString(), model.Repo.Name.ValueString(),
				model.TargetBranch.ValueString(), err.Error()),
		)
		return
	}

	model.fromQueue(queue)
	model.setID()
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func (r *mergeQueueResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var model mergeQueueResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.UpdateQueue(ctx, model.toUpdateRequest())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating merge queue",
			fmt.Sprintf("Could not update merge queue %s/%s/%s branch %q: %s",
				model.Repo.Host.ValueString(), model.Repo.Owner.ValueString(), model.Repo.Name.ValueString(),
				model.TargetBranch.ValueString(), err.Error()),
		)
		return
	}

	queue, err := r.client.GetQueue(ctx, client.GetQueueRequest{
		Repo:         client.Repo{Host: model.Repo.Host.ValueString(), Owner: model.Repo.Owner.ValueString(), Name: model.Repo.Name.ValueString()},
		TargetBranch: model.TargetBranch.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading merge queue after update",
			fmt.Sprintf("Could not read merge queue %s/%s/%s branch %q after update: %s",
				model.Repo.Host.ValueString(), model.Repo.Owner.ValueString(), model.Repo.Name.ValueString(),
				model.TargetBranch.ValueString(), err.Error()),
		)
		return
	}

	model.fromQueue(queue)
	model.setID()
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func (r *mergeQueueResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var model mergeQueueResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteQueue(ctx, client.DeleteQueueRequest{
		Repo:         client.Repo{Host: model.Repo.Host.ValueString(), Owner: model.Repo.Owner.ValueString(), Name: model.Repo.Name.ValueString()},
		TargetBranch: model.TargetBranch.ValueString(),
	})
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 400 {
			resp.Diagnostics.AddError(
				"Cannot delete merge queue: queue is not empty",
				"The merge queue still has PRs in it. Set state = \"DRAINING\" and wait for the "+
					"queue to empty before running terraform destroy again.",
			)
			return
		}
		resp.Diagnostics.AddError(
			"Error deleting merge queue",
			fmt.Sprintf("Could not delete merge queue %s/%s/%s branch %q: %s",
				model.Repo.Host.ValueString(), model.Repo.Owner.ValueString(), model.Repo.Name.ValueString(),
				model.TargetBranch.ValueString(), err.Error()),
		)
	}
}

func (r *mergeQueueResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Format: {host}/{owner}/{name}/{target_branch}
	// SplitN with n=4 handles branch names that contain slashes (e.g., "feature/foo").
	parts := strings.SplitN(req.ID, "/", 4)
	if len(parts) != 4 || parts[0] == "" || parts[1] == "" || parts[2] == "" || parts[3] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected format {host}/{owner}/{name}/{target_branch}, got: %q", req.ID),
		)
		return
	}

	model := mergeQueueResourceModel{
		ID: types.StringValue(parts[0] + "/" + parts[1] + "/" + parts[2] + "/" + parts[3]),
		Repo: repoModel{
			Host:  types.StringValue(parts[0]),
			Owner: types.StringValue(parts[1]),
			Name:  types.StringValue(parts[2]),
		},
		TargetBranch:                types.StringValue(parts[3]),
		Mode:                        types.StringNull(),
		Concurrency:                 types.Int64Null(),
		State:                       types.StringNull(),
		TestingTimeoutMinutes:       types.Int64Null(),
		PendingFailureDepth:         types.Int64Null(),
		CanOptimisticallyMerge:      types.BoolNull(),
		Batch:                       types.BoolNull(),
		BatchingMaxWaitTimeMinutes:  types.Int64Null(),
		BatchingMinSize:             types.Int64Null(),
		MergeMethod:                 types.StringNull(),
		CommentsEnabled:             types.BoolNull(),
		CommandsEnabled:             types.BoolNull(),
		CreatePrsForTestingBranches: types.BoolNull(),
		StatusCheckEnabled:          types.BoolNull(),
		DirectMergeMode:             types.StringNull(),
		OptimizationMode:            types.StringNull(),
		BisectionConcurrency:        types.Int64Null(),
		RequiredStatuses:            types.ListNull(types.StringType),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}
