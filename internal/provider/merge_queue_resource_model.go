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

	// Fields with API defaults (Computed + Optional in schema).
	Mode        types.String `tfsdk:"mode"`
	Concurrency types.Int64  `tfsdk:"concurrency"`
	State       types.String `tfsdk:"state"`

	// Optional fields without API defaults.
	TestingTimeoutMinutes       types.Int64  `tfsdk:"testing_timeout_minutes"`
	PendingFailureDepth         types.Int64  `tfsdk:"pending_failure_depth"`
	CanOptimisticallyMerge      types.Bool   `tfsdk:"can_optimistically_merge"`
	Batch                       types.Bool   `tfsdk:"batch"`
	BatchingMaxWaitTimeMinutes  types.Int64  `tfsdk:"batching_max_wait_time_minutes"`
	BatchingMinSize             types.Int64  `tfsdk:"batching_min_size"`
	MergeMethod                 types.String `tfsdk:"merge_method"`
	CommentsEnabled             types.Bool   `tfsdk:"comments_enabled"`
	CommandsEnabled             types.Bool   `tfsdk:"commands_enabled"`
	CreatePrsForTestingBranches types.Bool   `tfsdk:"create_prs_for_testing_branches"`
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

// toUpdateRequest builds an UpdateQueueRequest from all non-identity fields in the model.
// Null/unknown Terraform values produce nil pointer fields, leaving them unchanged on the API side.
// A null required_statuses field sends deleteRequiredStatuses=true to revert to defaults.
func (m *mergeQueueResourceModel) toUpdateRequest() client.UpdateQueueRequest {
	req := client.UpdateQueueRequest{
		Repo: client.Repo{
			Host:  m.Repo.Host.ValueString(),
			Owner: m.Repo.Owner.ValueString(),
			Name:  m.Repo.Name.ValueString(),
		},
		TargetBranch: m.TargetBranch.ValueString(),
	}

	if !m.Mode.IsNull() && !m.Mode.IsUnknown() {
		v := m.Mode.ValueString()
		req.Mode = &v
	}
	if !m.Concurrency.IsNull() && !m.Concurrency.IsUnknown() {
		v := int(m.Concurrency.ValueInt64())
		req.Concurrency = &v
	}
	if !m.State.IsNull() && !m.State.IsUnknown() {
		v := m.State.ValueString()
		req.State = &v
	}
	if !m.TestingTimeoutMinutes.IsNull() && !m.TestingTimeoutMinutes.IsUnknown() {
		v := int(m.TestingTimeoutMinutes.ValueInt64())
		req.TestingTimeoutMinutes = &v
	}
	if !m.PendingFailureDepth.IsNull() && !m.PendingFailureDepth.IsUnknown() {
		v := int(m.PendingFailureDepth.ValueInt64())
		req.PendingFailureDepth = &v
	}
	if !m.CanOptimisticallyMerge.IsNull() && !m.CanOptimisticallyMerge.IsUnknown() {
		v := m.CanOptimisticallyMerge.ValueBool()
		req.CanOptimisticallyMerge = &v
	}
	if !m.Batch.IsNull() && !m.Batch.IsUnknown() {
		v := m.Batch.ValueBool()
		req.Batch = &v
	}
	if !m.BatchingMaxWaitTimeMinutes.IsNull() && !m.BatchingMaxWaitTimeMinutes.IsUnknown() {
		v := int(m.BatchingMaxWaitTimeMinutes.ValueInt64())
		req.BatchingMaxWaitTimeMinutes = &v
	}
	if !m.BatchingMinSize.IsNull() && !m.BatchingMinSize.IsUnknown() {
		v := int(m.BatchingMinSize.ValueInt64())
		req.BatchingMinSize = &v
	}
	if !m.MergeMethod.IsNull() && !m.MergeMethod.IsUnknown() {
		v := m.MergeMethod.ValueString()
		req.MergeMethod = &v
	}
	if !m.CommentsEnabled.IsNull() && !m.CommentsEnabled.IsUnknown() {
		v := m.CommentsEnabled.ValueBool()
		req.CommentsEnabled = &v
	}
	if !m.CommandsEnabled.IsNull() && !m.CommandsEnabled.IsUnknown() {
		v := m.CommandsEnabled.ValueBool()
		req.CommandsEnabled = &v
	}
	if !m.CreatePrsForTestingBranches.IsNull() && !m.CreatePrsForTestingBranches.IsUnknown() {
		v := m.CreatePrsForTestingBranches.ValueBool()
		req.CreatePrsForTestingBranches = &v
	}
	if !m.StatusCheckEnabled.IsNull() && !m.StatusCheckEnabled.IsUnknown() {
		v := m.StatusCheckEnabled.ValueBool()
		req.StatusCheckEnabled = &v
	}
	if !m.DirectMergeMode.IsNull() && !m.DirectMergeMode.IsUnknown() {
		v := m.DirectMergeMode.ValueString()
		req.DirectMergeMode = &v
	}
	if !m.OptimizationMode.IsNull() && !m.OptimizationMode.IsUnknown() {
		v := m.OptimizationMode.ValueString()
		req.OptimizationMode = &v
	}
	if !m.BisectionConcurrency.IsNull() && !m.BisectionConcurrency.IsUnknown() {
		v := int(m.BisectionConcurrency.ValueInt64())
		req.BisectionConcurrency = &v
	}

	// required_statuses: null means revert to branch protection / trunk.yaml defaults.
	if m.RequiredStatuses.IsNull() || m.RequiredStatuses.IsUnknown() {
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
// Identity fields (ID, Repo, TargetBranch) are intentionally not set here because the API does
// not return them in getQueue responses. Callers must set ID from the model's own Repo/TargetBranch
// values after calling fromQueue.
func (m *mergeQueueResourceModel) fromQueue(q *client.Queue) {
	m.Mode = types.StringValue(q.Mode)
	m.Concurrency = types.Int64Value(int64(q.Concurrency))
	m.State = types.StringValue(q.State)

	if q.TestingTimeoutMinutes != nil {
		m.TestingTimeoutMinutes = types.Int64Value(int64(*q.TestingTimeoutMinutes))
	} else {
		m.TestingTimeoutMinutes = types.Int64Null()
	}
	if q.PendingFailureDepth != nil {
		m.PendingFailureDepth = types.Int64Value(int64(*q.PendingFailureDepth))
	} else {
		m.PendingFailureDepth = types.Int64Null()
	}
	if q.CanOptimisticallyMerge != nil {
		m.CanOptimisticallyMerge = types.BoolValue(*q.CanOptimisticallyMerge)
	} else {
		m.CanOptimisticallyMerge = types.BoolNull()
	}
	if q.Batch != nil {
		m.Batch = types.BoolValue(*q.Batch)
	} else {
		m.Batch = types.BoolNull()
	}
	if q.BatchingMaxWaitTimeMinutes != nil {
		m.BatchingMaxWaitTimeMinutes = types.Int64Value(int64(*q.BatchingMaxWaitTimeMinutes))
	} else {
		m.BatchingMaxWaitTimeMinutes = types.Int64Null()
	}
	if q.BatchingMinSize != nil {
		m.BatchingMinSize = types.Int64Value(int64(*q.BatchingMinSize))
	} else {
		m.BatchingMinSize = types.Int64Null()
	}
	if q.MergeMethod != nil {
		m.MergeMethod = types.StringValue(*q.MergeMethod)
	} else {
		m.MergeMethod = types.StringNull()
	}
	if q.CommentsEnabled != nil {
		m.CommentsEnabled = types.BoolValue(*q.CommentsEnabled)
	} else {
		m.CommentsEnabled = types.BoolNull()
	}
	if q.CommandsEnabled != nil {
		m.CommandsEnabled = types.BoolValue(*q.CommandsEnabled)
	} else {
		m.CommandsEnabled = types.BoolNull()
	}
	if q.CreatePrsForTestingBranches != nil {
		m.CreatePrsForTestingBranches = types.BoolValue(*q.CreatePrsForTestingBranches)
	} else {
		m.CreatePrsForTestingBranches = types.BoolNull()
	}
	if q.StatusCheckEnabled != nil {
		m.StatusCheckEnabled = types.BoolValue(*q.StatusCheckEnabled)
	} else {
		m.StatusCheckEnabled = types.BoolNull()
	}
	if q.DirectMergeMode != nil {
		m.DirectMergeMode = types.StringValue(*q.DirectMergeMode)
	} else {
		m.DirectMergeMode = types.StringNull()
	}
	if q.OptimizationMode != nil {
		m.OptimizationMode = types.StringValue(*q.OptimizationMode)
	} else {
		m.OptimizationMode = types.StringNull()
	}
	if q.BisectionConcurrency != nil {
		m.BisectionConcurrency = types.Int64Value(int64(*q.BisectionConcurrency))
	} else {
		m.BisectionConcurrency = types.Int64Null()
	}

	if len(q.RequiredStatuses) > 0 {
		elems := make([]attr.Value, len(q.RequiredStatuses))
		for i, s := range q.RequiredStatuses {
			elems[i] = types.StringValue(s)
		}
		m.RequiredStatuses, _ = types.ListValue(types.StringType, elems)
	} else {
		m.RequiredStatuses = types.ListNull(types.StringType)
	}
}
