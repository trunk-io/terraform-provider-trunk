# Contributing to Terraform Provider for Trunk.io

Thank you for your interest in contributing! This guide will help you get started.

## Reporting Bugs

Please [open a bug report](https://github.com/trunk-io/terraform-provider-trunk/issues/new?template=bug_report.yml) using our issue template. Include your Terraform and provider versions, the resource affected, and steps to reproduce the issue.

## Requesting Features

[Open a feature request](https://github.com/trunk-io/terraform-provider-trunk/issues/new?template=feature_request.yml) describing your use case and the behavior you'd like to see.

## Development Setup

### Requirements

- [Go](https://golang.org/dl/) >= 1.24
- [Terraform](https://www.terraform.io/downloads) >= 1.0

### Build

```bash
make build
```

### Run Tests

```bash
# Unit tests (no API key required)
make test

# Acceptance tests (requires a real API key and test repository)
TF_ACC=1 TRUNK_API_KEY=<key> make testacc
```

### Local Testing with Terraform

Build the provider and use `dev_overrides` to test against a real Terraform config without publishing:

```bash
go build -o terraform-provider-trunk .
```

Add to `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "registry.terraform.io/trunk-io/trunk" = "/path/to/terraform-provider-trunk"
  }
  direct {}
}
```

Then run `terraform plan` in any example directory — no `terraform init` needed with `dev_overrides`.

## Submitting Changes

1. Fork the repository and create a feature branch from `main`.
2. Make your changes, following existing code style (`make fmt` to format, `make lint` to lint).
3. Add or update tests for your changes.
4. Ensure all tests pass (`make test`).
5. Update `CHANGELOG.md` under the **Unreleased** section.
6. Open a pull request against `main`.

### Pull Request Guidelines

- Keep PRs focused — one feature or fix per PR.
- Include a clear description of what changed and why.
- Link any related issues.

## Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

## License

By contributing, you agree that your contributions will be licensed under the [MPL-2.0 License](LICENSE).
