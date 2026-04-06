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
		Description: "Interact with Trunk.io services.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				Description: "API key for Trunk.io. Can also be set via the TRUNK_API_KEY environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
			"base_url": schema.StringAttribute{
				Description: "Base URL for the Trunk API. Defaults to https://api.trunk.io/v1. Can also be set via the TRUNK_BASE_URL environment variable.",
				Optional:    true,
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
			"The provider cannot create the Trunk API client because there is no API key. "+
				"Set the api_key value in the provider configuration or use the TRUNK_API_KEY environment variable.",
		)
		return
	}

	baseURL := config.BaseURL.ValueString()
	if baseURL == "" {
		baseURL = os.Getenv("TRUNK_BASE_URL")
	}
	if baseURL == "" {
		baseURL = "https://api.trunk.io/v1"
	}
	c := client.NewClient(apiKey, baseURL)
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
