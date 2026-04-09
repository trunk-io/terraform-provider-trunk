package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/trunk-io/terraform-provider-trunk/internal/client"
)

// repoModel is the Terraform schema model for the repo nested attribute.
type repoModel struct {
	Host  types.String `tfsdk:"host"`
	Owner types.String `tfsdk:"owner"`
	Name  types.String `tfsdk:"name"`
}

// mergeQueueResourceModel is the Terraform schema model for the trunk_merge_queue resource.
type mergeQueueResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Repo         repoModel    `tfsdk:"repo"`
	TargetBranch types.String `tfsdk:"target_branch"`

	Mode                        types.String `tfsdk:"mode"`
	Concurrency                 types.Int64  `tfsdk:"concurrency"`
	State                       types.String `tfsdk:"state"`
	TestingTimeoutMinutes       types.Int64  `tfsdk:"testing_timeout_minutes"`
	PendingFailureDepth         types.Int64  `tfsdk:"pending_failure_depth"`
	CanOptimisticallyMerge      types.Bool   `tfsdk:"can_optimistically_merge"`
	Batch                       types.Bool   `tfsdk:"batch"`
	BatchingMaxWaitTimeMinutes  types.Int64  `tfsdk:"batching_max_wait_time_minutes"`
	BatchingMinSize             types.Int64  `tfsdk:"batching_min_size"`
	CreatePrsForTestingBranches types.Bool   `tfsdk:"create_prs_for_testing_branches"`
	MergeMethod                 types.String `tfsdk:"merge_method"`
	CommentsEnabled             types.Bool   `tfsdk:"comments_enabled"`
	CommandsEnabled             types.Bool   `tfsdk:"commands_enabled"`
	StatusCheckEnabled          types.Bool   `tfsdk:"status_check_enabled"`
	DirectMergeMode             types.String `tfsdk:"direct_merge_mode"`
	OptimizationMode            types.String `tfsdk:"optimization_mode"`
	BisectionConcurrency        types.Int64  `tfsdk:"bisection_concurrency"`
	RequiredStatuses            types.List   `tfsdk:"required_statuses"`
}

// toCreateRequest builds a CreateQueueRequest from the model's identity and mode/concurrency fields.
func (m *mergeQueueResourceModel) toCreateRequest() client.CreateQueueRequest {
	return client.CreateQueueRequest{
		Repo: client.Repo{
			Host:  m.Repo.Host.ValueString(),
			Owner: m.Repo.Owner.ValueString(),
			Name:  m.Repo.Name.ValueString(),
		},
		TargetBranch: m.TargetBranch.ValueString(),
		Mode:         m.Mode.ValueString(),
		Concurrency:  int(m.Concurrency.ValueInt64()),
	}
}

// toUpdateRequest builds an UpdateQueueRequest. The receiver (m) provides the values
// to send (typically from the plan). The config parameter determines which fields the
// user explicitly set — only those are included in the request. Fields that are null in
// the config are server-computed defaults and are omitted so the API leaves them unchanged.
func (m *mergeQueueResourceModel) toUpdateRequest(config *mergeQueueResourceModel) client.UpdateQueueRequest {
	req := client.UpdateQueueRequest{
		Repo: client.Repo{
			Host:  m.Repo.Host.ValueString(),
			Owner: m.Repo.Owner.ValueString(),
			Name:  m.Repo.Name.ValueString(),
		},
		TargetBranch: m.TargetBranch.ValueString(),
	}

	if !config.Mode.IsNull() {
		v := m.Mode.ValueString()
		req.Mode = &v
	}
	if !config.Concurrency.IsNull() {
		v := int(m.Concurrency.ValueInt64())
		req.Concurrency = &v
	}
	if !config.State.IsNull() {
		v := m.State.ValueString()
		req.State = &v
	}
	if !config.TestingTimeoutMinutes.IsNull() {
		v := int(m.TestingTimeoutMinutes.ValueInt64())
		req.TestingTimeoutMinutes = &v
	}
	if !config.PendingFailureDepth.IsNull() {
		v := int(m.PendingFailureDepth.ValueInt64())
		req.PendingFailureDepth = &v
	}
	if !config.CanOptimisticallyMerge.IsNull() {
		v := m.CanOptimisticallyMerge.ValueBool()
		req.CanOptimisticallyMerge = &v
	}
	if !config.Batch.IsNull() {
		v := m.Batch.ValueBool()
		req.Batch = &v
	}
	if !config.BatchingMaxWaitTimeMinutes.IsNull() {
		v := int(m.BatchingMaxWaitTimeMinutes.ValueInt64())
		req.BatchingMaxWaitTimeMinutes = &v
	}
	if !config.BatchingMinSize.IsNull() {
		v := int(m.BatchingMinSize.ValueInt64())
		req.BatchingMinSize = &v
	}
	if !config.MergeMethod.IsNull() {
		v := m.MergeMethod.ValueString()
		req.MergeMethod = &v
	}
	if !config.CommentsEnabled.IsNull() {
		v := m.CommentsEnabled.ValueBool()
		req.CommentsEnabled = &v
	}
	if !config.CommandsEnabled.IsNull() {
		v := m.CommandsEnabled.ValueBool()
		req.CommandsEnabled = &v
	}
	if !config.CreatePrsForTestingBranches.IsNull() {
		v := m.CreatePrsForTestingBranches.ValueBool()
		req.CreatePrsForTestingBranches = &v
	}
	if !config.StatusCheckEnabled.IsNull() {
		v := m.StatusCheckEnabled.ValueBool()
		req.StatusCheckEnabled = &v
	}
	if !config.DirectMergeMode.IsNull() {
		v := m.DirectMergeMode.ValueString()
		req.DirectMergeMode = &v
	}
	if !config.OptimizationMode.IsNull() {
		v := m.OptimizationMode.ValueString()
		req.OptimizationMode = &v
	}
	if !config.BisectionConcurrency.IsNull() {
		v := int(m.BisectionConcurrency.ValueInt64())
		req.BisectionConcurrency = &v
	}

	// required_statuses: null in config means revert to branch protection / trunk.yaml defaults.
	if config.RequiredStatuses.IsNull() {
		t := true
		req.DeleteRequiredStatuses = &t
	} else {
		statuses := make([]string, 0, len(m.RequiredStatuses.Elements()))
		for _, elem := range m.RequiredStatuses.Elements() {
			statuses = append(statuses, elem.(types.String).ValueString())
		}
		req.RequiredStatuses = &statuses
	}

	return req
}

// setID computes and sets the id attribute from the model's identity fields.
// Must be called after identity fields (Repo, TargetBranch) are already set.
func (m *mergeQueueResourceModel) setID() {
	m.ID = types.StringValue(
		m.Repo.Host.ValueString() + "/" +
			m.Repo.Owner.ValueString() + "/" +
			m.Repo.Name.ValueString() + "/" +
			m.TargetBranch.ValueString(),
	)
}

// fromQueue populates the model from the API queue response.
// All fields are set unconditionally from the API response.
// Identity fields (ID, Repo, TargetBranch) are not set here; callers must
// set ID via setID after calling fromQueue.
func (m *mergeQueueResourceModel) fromQueue(q *client.Queue) {
	m.Mode = types.StringValue(q.Mode)
	m.Concurrency = types.Int64Value(int64(q.Concurrency))
	m.State = types.StringValue(q.State)
	m.TestingTimeoutMinutes = types.Int64Value(int64(q.TestingTimeoutMinutes))
	m.PendingFailureDepth = types.Int64Value(int64(q.PendingFailureDepth))
	m.CanOptimisticallyMerge = types.BoolValue(q.CanOptimisticallyMerge)
	m.Batch = types.BoolValue(q.Batch)
	m.BatchingMaxWaitTimeMinutes = types.Int64Value(int64(q.BatchingMaxWaitTimeMinutes))
	m.BatchingMinSize = types.Int64Value(int64(q.BatchingMinSize))
	m.CreatePrsForTestingBranches = types.BoolValue(q.CreatePrsForTestingBranches)
	m.MergeMethod = types.StringValue(q.MergeMethod)
	m.CommentsEnabled = types.BoolValue(q.CommentsEnabled)
	m.CommandsEnabled = types.BoolValue(q.CommandsEnabled)
	m.StatusCheckEnabled = types.BoolValue(q.StatusCheckEnabled)
	m.DirectMergeMode = types.StringValue(q.DirectMergeMode)
	m.OptimizationMode = types.StringValue(q.OptimizationMode)
	m.BisectionConcurrency = types.Int64Value(int64(q.BisectionConcurrency))

	if q.RequiredStatuses != nil {
		elems := make([]attr.Value, len(*q.RequiredStatuses))
		for i, s := range *q.RequiredStatuses {
			elems[i] = types.StringValue(s)
		}
		m.RequiredStatuses, _ = types.ListValue(types.StringType, elems)
	} else {
		m.RequiredStatuses = types.ListNull(types.StringType)
	}
}
