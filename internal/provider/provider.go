package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/trunk-io/terraform-provider-trunk/internal/client"
)

type trunkProvider struct {
	version string
}

// New returns a factory function that creates a new trunkProvider instance.
// The version string is embedded in provider metadata for registry display.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &trunkProvider{
			version: version,
		}
	}
}

func (p *trunkProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "trunk"
	resp.Version = p.version
}

func (p *trunkProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage Trunk merge queues.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				Description: "A Trunk API key (`trunk_v2_...`, with the **Update Merge Queue settings** permission), which uses the v2 API, " +
					"or the organization API token, which uses the v1 API. Can also be set with the `TRUNK_API_KEY` environment variable.",
				Optional:  true,
				Sensitive: true,
			},
			"base_url": schema.StringAttribute{
				Description: "The Trunk API host. Defaults to `https://api.trunk.io`. A trailing `/v1` or `/v2` is ignored, since the " +
					"provider picks the version from the credential. Can also be set with the `TRUNK_BASE_URL` environment variable.",
				Optional: true,
			},
		},
	}
}

func (p *trunkProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config struct {
		APIKey  types.String `tfsdk:"api_key"`
		BaseURL types.String `tfsdk:"base_url"`
	}

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey := config.APIKey.ValueString()
	if apiKey == "" {
		apiKey = os.Getenv("TRUNK_API_KEY")
	}
	if apiKey == "" {
		resp.Diagnostics.AddError(
			"Missing API Key",
			"The provider needs a Trunk API key or the organization API token. "+
				"Set api_key in the provider configuration or the TRUNK_API_KEY environment variable.",
		)
		return
	}

	baseURL := config.BaseURL.ValueString()
	if baseURL == "" {
		baseURL = os.Getenv("TRUNK_BASE_URL")
	}
	c := client.NewClient(apiKey, baseURL, p.version)
	resp.ResourceData = c
	resp.DataSourceData = c
}

func (p *trunkProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewMergeQueueResource,
	}
}

func (p *trunkProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}
