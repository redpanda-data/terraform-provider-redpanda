# Manual validation (live cluster smoke test)

When automated test tiers (model unit, colocated integration, colocated acc) aren't enough — drift detection, provider-upgrade compatibility, real API behavior. Delegates most of the recipe to the user-level `manual-test-redpanda-resource` skill; this file is the project-side index.

## When manual validation is required

- **Add flow:** after Tier 2 (integration) passes and the resource compiles end-to-end. Validates that the resource actually round-trips against Redpanda Cloud — integration tier uses bufconn fakes and can miss real-server behavior (rate limits, async timing, server-enforced constraints).
- **Extend flow:** the §6 upgrade scenario (steps 0a/0b/0c) is **load-bearing** — only manual testing can validate that existing user state plans cleanly against HEAD with the new field absent from config.

The integration tier (Tier 2) now pre-empts most CRUD coverage that used to require live cluster work. Manual validation focuses on:

1. Drift detection (out-of-band mutation → plan detects)
2. Upgrade scenario (released-provider state → HEAD-provider plan)
3. Real API behavior the fake doesn't model

## Workdir & journal

```bash
mkdir -p manual-tests/<name>          # gitignored
cd manual-tests/<name>
# Each cycle gets a versioned suffix: <name>-v2, <name>-v3, ...
```

Per memory `feedback_summary_md_detail.md`: append a detailed entry to `manual-tests/SUMMARY.md` after every cycle covering scope / setup / cycle log / new findings / cross-cycle status / resources alive / next-cycle gating. Tag extend cycles as "extension test."

## Cluster strategy

Per memories `feedback_cluster_reuse.md` and `feedback_keep_cluster_alive.md`: **reuse first.**

- **Dataplane resources** (topic, user, acl, schema, serviceaccount, pipeline) — always reuse an existing `tfrp-*` cluster. `task cleanup:redpanda:dry` lists them.
- **Infra resources** (cluster, network, resource_group, BYOVPC) — fresh creation IS the test. Even then, keep the cluster alive across cycles unless the cycle specifically tests destroy.
- **Don't destroy between cycles** — saves ~50 min and $10–15 per cycle.

For add-flow new dataplane resources, reuse a cluster. For add-flow new infra resources, fresh. For extend-flow on existing infra resources (cluster, network), reuse — the new field is the test, not the resource lifecycle.

## Credentials

Per memory `reference_redpanda_test_creds.md`:

```bash
export REDPANDA_CLIENT_ID=<authorized id>
export REDPANDA_CLIENT_SECRET=<authorized secret>
export REDPANDA_CLOUD_ENVIRONMENT=pre    # required — defaults to prod otherwise
# Plus AWS/GCP creds per target
```

The exact preprod credentials are in memory. **Don't proceed without `REDPANDA_CLOUD_ENVIRONMENT=pre`** — production credentials in a test cycle is a memory-load away from accidental real-customer impact.

## Dev overrides wiring

Per memory `reference_local_dev_provider.md` — the dev-override key is `hashicorp/redpanda`, **not** `redpanda-data/redpanda`. The registry source is `redpanda-data/redpanda` but the local binary defaults to `hashicorp/redpanda`. Get this wrong and `terraform plan` won't pick up local changes.

```bash
# Build local binary
go build -o terraform-provider-redpanda .

# .terraformrc points at the binary's *directory*
cat > ~/.terraformrc <<'EOF'
provider_installation {
  dev_overrides {
    "hashicorp/redpanda" = "/Users/gene/GolandProjects/terraform-provider-redpanda"
  }
  direct {}
}
EOF
```

Sanity check: `terraform plan` must print **"Provider development overrides are in effect"**. If it doesn't, the override path is wrong.

Alternative: `task build:install` installs to the local TF plugin cache, no `.terraformrc` needed.

## When a cycle hangs or leaves resources

Per memory `feedback_use_taskfile_cleanup.md`: kill the terraform process and run `task cleanup:aws:ci` / `cleanup:gcp:ci` / `task cleanup:redpanda` directly. Don't wait for `terraform destroy` to recover — it usually can't once state is wedged.

## Standard CRUD sequence (add flow)

1. `terraform plan` (sanity, expect creates)
2. `terraform apply` (initial create)
3. Feature validation (rpk/console/API direct check — verify the thing actually exists upstream)
4. `terraform plan` — expect **No changes**
5. Update a mutable attribute → `apply`
6. `terraform plan` — expect **No changes**
7. `terraform state rm <addr>`
8. `terraform import <addr> <id>`
9. `terraform plan` — expect **No changes**
10. Drift: mutate out-of-band → `plan` must detect

Skip §10 for resources without ImportState. Skip §5–§6 for resources without mutable attributes.

## Upgrade scenario (extend flow — load-bearing)

The §6 from the manual-test skill. Cannot be replaced by any automated test — only this exercises real released-provider state with HEAD code.

**0a. Apply with released provider, config WITHOUT the new field.**
```bash
# Comment out dev_overrides in ~/.terraformrc, or use a separate workdir
terraform init  # downloads latest released provider
terraform apply # creates resource with old schema
```

**0b. Switch to dev binary, replace provider in state, re-plan.**
```bash
# Re-enable dev_overrides
terraform state replace-provider registry.terraform.io/redpanda-data/redpanda hashicorp/redpanda
terraform plan
# MUST report "No changes." Any diff = backwards-compat break.
```

**0c. Add the new field to config, apply.**
```bash
# Edit main.tf to set the new field
terraform apply
terraform plan  # No changes
```

What "no spurious diff" means in 0b varies by field shape:

- `optional+computed` — should anchor null-in-state via `UseStateForUnknown`; no diff.
- `computed_only` — appears as `(known after apply)` on first refresh, then stable; no diff after.
- `extra: true` — null in old state. Default value must match null. Non-null default = backwards-compat bug.

The `terraform state replace-provider` step is the most common trip-hazard — without it, the plan fails with "Missing required provider."

## Datasource cycle

For new resources that also have a datasource: bundle the datasource test in the same workdir.

- Two `terraform plan -detailed-exitcode` runs to verify stability
- No mutation/destroy/import for datasources — they're read-only

## Bundling with the manual-test skill

When you invoke this skill's flow, the user-level `manual-test-redpanda-resource` skill (at `~/.claude/skills/manual-test-redpanda-resource/`) is the canonical step-by-step recipe with templates. The project skills delegate to it rather than duplicating.

## See also

- [testing-tiers](testing-tiers.md) — what manual validation does and doesn't replace
- `~/.claude/skills/manual-test-redpanda-resource/SKILL.md` — the full recipe
