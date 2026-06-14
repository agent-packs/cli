---
name: infrastructure-as-code
description: Provision and manage cloud infrastructure with Terraform, Pulumi, or similar IaC tools. Use when writing infrastructure definitions, planning changes, or managing state safely.
license: Apache-2.0
compatibility: Works with coding agents that support Agent Skills.
metadata:
  agentpacks.version: "0.1.0"
---

# Infrastructure as Code

Infrastructure changes are code changes. Apply the same review, testing, and rollback discipline.

## Module Design

- Keep modules small and single-purpose. A module for "VPC" should not also manage IAM roles.
- Accept all environment-specific values as input variables with types. No hardcoded region, account ID, or CIDR in a module body.
- Output only what callers need — don't expose internal resource IDs unless they're consumed upstream.
- Name resources with `${var.env}-${var.app}-${purpose}` so resources are identifiable in cloud consoles without logging in to Terraform.

## State Management

- Use remote state (S3 + DynamoDB lock, GCS, Terraform Cloud). Local `terraform.tfstate` in a shared repo causes conflicts.
- Use workspaces or separate state files per environment — never share state between prod and staging.
- Never edit state manually (`terraform state mv/rm`) without a paired `terraform plan` showing the effect.
- Enable state versioning and set a lifecycle policy to retain 30+ versions.

## Plan Before Apply

- Always run `terraform plan -out=tfplan` and review the output before `apply`. Check for unexpected destroys.
- A `-/+` line (destroy-then-create) on a load balancer, database, or DNS record is a production incident waiting to happen — understand why before proceeding.
- Use `terraform plan` in CI on every PR targeting infrastructure. Block merge on errors or diffs that weren't expected.
- For stateful resources (RDS, ElasticSearch), set `lifecycle { prevent_destroy = true }` and require explicit override to destroy.

## Secrets and Sensitive Values

- Never store secrets in `.tf` files or `terraform.tfvars` committed to version control.
- Use `sensitive = true` on output and variable blocks that carry secrets — Terraform will redact them in plan output.
- Source secret values from a secrets manager (AWS SSM Parameter Store, HashiCorp Vault, GCP Secret Manager) via data sources at apply time.

## Drift and Day-2

- Run `terraform plan` in CI on a schedule (daily) to detect drift from manual console changes.
- Tag every resource with `managed-by = "terraform"`, `env`, `team`, and `cost-center`.
- Use `terraform validate` and `tflint` in pre-commit hooks to catch syntax and provider-specific errors before push.

## Checklist

- [ ] Remote state configured with locking.
- [ ] `terraform plan` reviewed before every apply, especially for destroys.
- [ ] No secrets in `.tf` files or committed `tfvars`.
- [ ] Stateful resources have `prevent_destroy = true`.
- [ ] All resources tagged with env, team, and managed-by.
- [ ] CI runs `terraform plan` on every infrastructure PR.
