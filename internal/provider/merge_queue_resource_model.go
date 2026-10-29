package provider

import (
	"context"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/trunk-io/terraform-provider-trunk/internal/client"
)

// repoModel is the repo nested attribute.
type repoModel struct {
	Host  types.String `tfsdk:"host"`
	Owner types.String `tfsdk:"owner"`
	Name  types.String `tfsdk:"name"`
}

// batchingRuleModel is one element of batching_rules.
type batchingRuleModel struct {
	TargetPattern types.String `tfsdk:"target_pattern"`
	Policy        types.String `tfsdk:"policy"`
}

var batchingRuleAttrTypes = map[string]attr.Type{
	"target_pattern": types.StringType,
	"policy":         types.StringType,
}

var batchingRuleType = types.ObjectType{AttrTypes: batchingRuleAttrTypes}

// mergeQueueResourceModel is the trunk_merge_queue resource.
type mergeQueueResourceModel struct {
	ID           types.String `tfsdk:"id"`
	QueueID      types.String `tfsdk:"queue_id"`
	ManagedState types.String `tfsdk:"managed_state"`
	Repo         *repoModel   `tfsdk:"repo"`
	TargetBranch types.String `tfsdk:"target_branch"`

	Mode                        types.String `tfsdk:"mode"`
	State                       types.String `tfsdk:"state"`
	Concurrency                 types.Int64  `tfsdk:"concurrency"`
	BisectionConcurrency        types.Int64  `tfsdk:"bisection_concurrency"`
	TestingTimeoutMinutes       types.Int64  `tfsdk:"testing_timeout_minutes"`
	PendingFailureDepth         types.Int64  `tfsdk:"pending_failure_depth"`
	CanOptimisticallyMerge      types.Bool   `tfsdk:"can_optimistically_merge"`
	Batch                       types.Bool   `tfsdk:"batch"`
	BatchingMaxWaitTimeMinutes  types.Int64  `tfsdk:"batching_max_wait_time_minutes"`
	BatchingMinSize             types.Int64  `tfsdk:"batching_min_size"`
	BatchingRules               types.Set    `tfsdk:"batching_rules"`
	CreatePrsForTestingBranches types.Bool   `tfsdk:"create_prs_for_testing_branches"`
	MergeMethod                 types.String `tfsdk:"merge_method"`
	CommentsEnabled             types.Bool   `tfsdk:"comments_enabled"`
	CommandsEnabled             types.Bool   `tfsdk:"commands_enabled"`
	StatusCheckEnabled          types.Bool   `tfsdk:"status_check_enabled"`
	ExtensionEnabled            types.Bool   `tfsdk:"extension_enabled"`
	LabelCommandsEnabled        types.Bool   `tfsdk:"label_commands_enabled"`
	StateLabelsEnabled          types.Bool   `tfsdk:"state_labels_enabled"`
	EnqueueingLabel             types.String `tfsdk:"enqueueing_label"`
	DirectMergeMode             types.String `tfsdk:"direct_merge_mode"`
	OptimizationMode            types.String `tfsdk:"optimization_mode"`
	TestBranchConstructionMode  types.String `tfsdk:"test_branch_construction_mode"`
	NotReadyTimeoutHours        types.Int64  `tfsdk:"not_ready_timeout_hours"`
	RequiredStatuses            types.Set    `tfsdk:"required_statuses"`
}

func (m *mergeQueueResourceModel) repo() client.Repo {
	if m.Repo == nil {
		return client.Repo{}
	}
	return client.Repo{
		Host:  m.Repo.Host.ValueString(),
		Owner: m.Repo.Owner.ValueString(),
		Name:  m.Repo.Name.ValueString(),
	}
}

func (m *mergeQueueResourceModel) ref() client.QueueRef {
	ref := client.QueueRef{Repo: m.repo(), TargetBranch: m.TargetBranch.ValueString()}
	if !m.QueueID.IsNull() && !m.QueueID.IsUnknown() {
		ref.ID = m.QueueID.ValueString()
	}
	return ref
}

// Values to send. Only attributes set in the configuration are sent: an omitted attribute is
// left as it is on the queue, whoever set it.

func strIfSet(config, plan types.String) *string {
	if config.IsNull() || plan.IsUnknown() {
		return nil
	}
	v := plan.ValueString()
	return &v
}

func intIfSet(config, plan types.Int64) *int {
	if config.IsNull() || plan.IsUnknown() {
		return nil
	}
	v := int(plan.ValueInt64())
	return &v
}

func boolIfSet(config, plan types.Bool) *bool {
	if config.IsNull() || plan.IsUnknown() {
		return nil
	}
	v := plan.ValueBool()
	return &v
}

// settings collects the attributes set in config, with the planned values.
func (m *mergeQueueResourceModel) settings(ctx context.Context, config *mergeQueueResourceModel) (client.Settings, diag.Diagnostics) {
	var diags diag.Diagnostics
	s := client.Settings{
		Concurrency:                 intIfSet(config.Concurrency, m.Concurrency),
		BisectionConcurrency:        intIfSet(config.BisectionConcurrency, m.BisectionConcurrency),
		TestingTimeoutMinutes:       intIfSet(config.TestingTimeoutMinutes, m.TestingTimeoutMinutes),
		PendingFailureDepth:         intIfSet(config.PendingFailureDepth, m.PendingFailureDepth),
		CanOptimisticallyMerge:      boolIfSet(config.CanOptimisticallyMerge, m.CanOptimisticallyMerge),
		Batch:                       boolIfSet(config.Batch, m.Batch),
		BatchingMaxWaitTimeMinutes:  intIfSet(config.BatchingMaxWaitTimeMinutes, m.BatchingMaxWaitTimeMinutes),
		BatchingMinSize:             intIfSet(config.BatchingMinSize, m.BatchingMinSize),
		CreatePrsForTestingBranches: boolIfSet(config.CreatePrsForTestingBranches, m.CreatePrsForTestingBranches),
		MergeMethod:                 strIfSet(config.MergeMethod, m.MergeMethod),
		CommentsEnabled:             boolIfSet(config.CommentsEnabled, m.CommentsEnabled),
		CommandsEnabled:             boolIfSet(config.CommandsEnabled, m.CommandsEnabled),
		StatusCheckEnabled:          boolIfSet(config.StatusCheckEnabled, m.StatusCheckEnabled),
		ExtensionEnabled:            boolIfSet(config.ExtensionEnabled, m.ExtensionEnabled),
		LabelCommandsEnabled:        boolIfSet(config.LabelCommandsEnabled, m.LabelCommandsEnabled),
		StateLabelsEnabled:          boolIfSet(config.StateLabelsEnabled, m.StateLabelsEnabled),
		EnqueueingLabel:             strIfSet(config.EnqueueingLabel, m.EnqueueingLabel),
		DirectMergeMode:             strIfSet(config.DirectMergeMode, m.DirectMergeMode),
		OptimizationMode:            strIfSet(config.OptimizationMode, m.OptimizationMode),
		TestBranchConstructionMode:  strIfSet(config.TestBranchConstructionMode, m.TestBranchConstructionMode),
		NotReadyTimeoutHours:        intIfSet(config.NotReadyTimeoutHours, m.NotReadyTimeoutHours),
	}
	if !config.RequiredStatuses.IsNull() && !m.RequiredStatuses.IsUnknown() {
		statuses := []string{}
		diags.Append(m.RequiredStatuses.ElementsAs(ctx, &statuses, false)...)
		s.RequiredStatuses = &statuses
	}
	if !config.BatchingRules.IsNull() && !m.BatchingRules.IsUnknown() {
		var rules []batchingRuleModel
		diags.Append(m.BatchingRules.ElementsAs(ctx, &rules, false)...)
		out := make([]client.BatchingRule, 0, len(rules))
		for _, r := range rules {
			out = append(out, client.BatchingRule{TargetPattern: r.TargetPattern.ValueString(), Policy: r.Policy.ValueString()})
		}
		s.BatchingRules = &out
	}
	return s, diags
}

func (m *mergeQueueResourceModel) createParams(ctx context.Context, config *mergeQueueResourceModel) (client.CreateParams, diag.Diagnostics) {
	s, diags := m.settings(ctx, config)
	return client.CreateParams{
		Repo:         m.repo(),
		TargetBranch: m.TargetBranch.ValueString(),
		Mode:         strIfSet(config.Mode, m.Mode),
		State:        strIfSet(config.State, m.State),
		Settings:     s,
	}, diags
}

func (m *mergeQueueResourceModel) updateParams(ctx context.Context, config, prior *mergeQueueResourceModel) (client.UpdateParams, diag.Diagnostics) {
	s, diags := m.settings(ctx, config)
	ref := m.ref()
	if ref.ID == "" {
		ref.ID = prior.ref().ID
	}
	return client.UpdateParams{
		Ref:      ref,
		Mode:     strIfSet(config.Mode, m.Mode),
		State:    strIfSet(config.State, m.State),
		Settings: s,
	}, diags
}

func nullableString(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

// fromQueue sets the model from the API's view of the queue.
//
// While a queue switches modes, the API reports its old mode and the state switching_modes until
// the switch finishes. In that window this keeps the mode and state from expected (the plan just
// applied, or the prior state on a read), so Terraform neither sees the change as failed nor
// sends a state change that would abandon the switch. It reports whether that happened.
func (m *mergeQueueResourceModel) fromQueue(q *client.Queue, expected *mergeQueueResourceModel) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics

	m.Repo = &repoModel{
		Host:  types.StringValue(q.Repo.Host),
		Owner: types.StringValue(q.Repo.Owner),
		Name:  types.StringValue(q.Repo.Name),
	}
	m.TargetBranch = types.StringValue(q.TargetBranch)
	m.ID = types.StringValue(q.Repo.Host + "/" + q.Repo.Owner + "/" + q.Repo.Name + "/" + q.TargetBranch)
	m.QueueID = nullableString(q.ID)
	m.ManagedState = nullableString(q.ManagedState)

	m.Mode = types.StringValue(q.Mode)
	m.State = types.StringValue(q.State)
	switching := q.State == client.StateSwitchingModes
	if switching && expected != nil {
		if known(expected.Mode) {
			m.Mode = expected.Mode
		}
		if known(expected.State) && expected.State.ValueString() != client.StateSwitchingModes {
			m.State = expected.State
		}
	}

	m.Concurrency = types.Int64Value(int64(q.Concurrency))
	m.BisectionConcurrency = types.Int64Value(int64(q.BisectionConcurrency))
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
	m.ExtensionEnabled = types.BoolValue(q.ExtensionEnabled)
	m.LabelCommandsEnabled = types.BoolValue(q.LabelCommandsEnabled)
	m.StateLabelsEnabled = types.BoolValue(q.StateLabelsEnabled)
	m.EnqueueingLabel = types.StringValue(q.EnqueueingLabel)
	m.DirectMergeMode = types.StringValue(q.DirectMergeMode)
	m.OptimizationMode = types.StringValue(q.OptimizationMode)
	m.TestBranchConstructionMode = types.StringValue(q.TestBranchConstructionMode)
	m.NotReadyTimeoutHours = types.Int64Value(int64(q.NotReadyTimeoutHours))

	statuses := append([]string(nil), q.RequiredStatuses...)
	sort.Strings(statuses)
	elems := make([]attr.Value, 0, len(statuses))
	for _, s := range statuses {
		elems = append(elems, types.StringValue(s))
	}
	set, d := types.SetValue(types.StringType, elems)
	diags.Append(d...)
	m.RequiredStatuses = set

	rules := make([]attr.Value, 0, len(q.BatchingRules))
	for _, r := range q.BatchingRules {
		obj, d := types.ObjectValue(batchingRuleAttrTypes, map[string]attr.Value{
			"target_pattern": types.StringValue(r.TargetPattern),
			"policy":         types.StringValue(r.Policy),
		})
		diags.Append(d...)
		rules = append(rules, obj)
	}
	ruleSet, d := types.SetValue(batchingRuleType, rules)
	diags.Append(d...)
	m.BatchingRules = ruleSet

	return switching, diags
}

func known(v types.String) bool {
	return !v.IsNull() && !v.IsUnknown()
}
