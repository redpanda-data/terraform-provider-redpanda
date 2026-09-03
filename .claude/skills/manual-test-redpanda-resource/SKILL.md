---
name: manual-test-redpanda-resource
description: Use when manually validating a terraform-provider-redpanda resource or datasource against a live Redpanda Cloud cluster. Triggers on "manual test", "smoke test", "validate redpanda_X end-to-end", "run terraform apply against a real cluster", or as the live-cluster step of the add-redpanda-resource and extend-redpanda-resource skills. Also use for refactor / migration branches (e.g. schema-gen-style work that re-emits a resource's Flatten/Expand from generated code) — the §6 upgrade scenario is the load-bearing assertion in that case. Asks the user about reusing an existing test cluster up front; defaults to AWS dedicated when creating fresh; covers CRUD, no-op plan, state rm + import, drift detection, and (for extension / refactor cases) the upgrade-scenario pre-step.
---

# Manual smoke test against a live Redpanda Cloud cluster

Use this skill when validating a terraform-provider-redpanda change against a real cluster. Automated tests miss issues that only surface live: drift on no-op plans, broken imports, unstable Read paths, plan-modifier mistakes, schema-evolution traps. Skipping is a common source of "PR merged, then a bug ships next release."

This skill is invoked:

- **Standalone** to validate something without the full add/extend flow (e.g. confirming a bug fix, sanity-checking a release candidate).
- As the live-cluster step of **`add-redpanda-resource`** (greenfield resource flow).
- As the live-cluster step of **`extend-redpanda-resource`** (adding a field) — that flow asks you to also run the **§6 upgrade scenario** here.
- **For refactor / migration branches** (Flatten/Expand regenerated from schemagen, hand-written models deleted, alias deprecations introduced). For these, the §6 upgrade scenario is the **single most important check** in the entire run — it's the only way to detect schema-shape drift against state written by the released provider. Treat §6 as required, not optional.

## 1. Cluster strategy: reuse first

Standing up a dedicated cluster takes ~10–20 minutes and burns real cloud spend. Reuse a long-lived test cluster whenever the change doesn't drive cluster lifecycle.

| Resource type | Strategy |
|---------------|----------|
| Dataplane resource (topic, user, acl, role, schema, schema_registry_acl, role_assignment) | **Reuse.** Targets `cluster_api_url` of any compatible existing cluster. |
| Pipeline | **Reuse.** Targets a cluster. |
| Datasource that reads existing state (cluster, network, resource_group, throughput_tiers, regions) | **Reuse.** Just reads. |
| Network, resource_group (as resources) | **Fresh.** They underlie the cluster; reusing doesn't make sense. |
| Cluster, serverless_cluster, BYOC, BYOVPC, serverless_private_link | **Fresh.** The test IS the cluster lifecycle. |

**Before doing anything else, ask the user:**

> "I'd like to reuse an existing test cluster for this manual smoke test. Do you have one I should target? Paste the `cluster_id` (and `cluster_api_url` if you know it), or say 'new' to spin up a fresh one."

Don't assume. If they paste creds for an existing cluster, plumb them as `var.cluster_id` (or hardcode `cluster_api_url` for dataplane resources) and skip the cluster-create step in the test plan.

If they don't have one and the resource is reuse-eligible, offer to spin up a cluster *once*, run the test, then leave it standing for the next test (warning them about ongoing cost). They decide.

**To find existing test clusters** if the user is unsure what's reusable, list them with the dry-run sweeper (which only enumerates, doesn't delete):

```bash
REDPANDA_CLOUD_ENVIRONMENT=pre task cleanup:redpanda:dry
```

That prints all `tfrp-*`-prefixed clusters in the preprod env. If something's been left up, ask the user before reusing it — it may belong to another in-flight test.

## 2. Pick the target (when creating fresh)

| Decision | Default | When to override |
|----------|---------|------------------|
| Cloud provider | **AWS** | Resource is provider-specific (e.g. AWS-only PrivateLink, GCP-only Workload Identity). |
| Cluster type | **dedicated** | Resource is serverless-only (e.g. `serverless_private_link`) or BYOC-only. |
| Region | match the example you're starting from (e.g. `us-west-2` for `examples/cluster/aws/main.tf`) | Quota constraints. |

## 3. Credentials

Required env vars:

- `REDPANDA_CLIENT_ID`, `REDPANDA_CLIENT_SECRET` — Redpanda Cloud
- `REDPANDA_CLOUD_ENVIRONMENT` — set to `pre` for preprod (the preprod test creds fail auth against prod)
- AWS: an `AWS_PROFILE` for the test account (SSO profiles need `aws sso login` first; the user must run it), or `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` (and `AWS_SESSION_TOKEN` if assumed-role)
- GCP: `GOOGLE_APPLICATION_CREDENTIALS` pointing at a service-account JSON key
- Azure: `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`, `AZURE_TENANT_ID`, `AZURE_SUBSCRIPTION_ID`

Preprod runs require `REDPANDA_CLIENT_ID` / `REDPANDA_CLIENT_SECRET` for the preprod org with `REDPANDA_CLOUD_ENVIRONMENT=pre`; pass them via exported vars in a script file, never inline on the command line. If they are not already in the environment: **stop and ask the user.** Don't assume defaults, don't try to read AWS profiles you weren't told about, don't skip the manual test. Use a clear prompt naming exactly which vars you need.

## 4. Set up the work directory

```bash
mkdir -p manual-tests/<name>
grep -q '^manual-tests/' .gitignore || echo 'manual-tests/' >> .gitignore
```

**Versioned workdir naming.** For cycles you'll run more than once (refactor-branch re-tests, upgrade-scenario reruns after a fix lands), suffix the workdir with a version: `manual-tests/<name>-v2/`, `-v3/`, `-v4b/`, etc. Use a matching `tfrp-<name>-vN-*` resource name prefix so the sweep tool and your live state never collide with leftovers from prior cycles (especially when one cycle's destroy timed out client-side but server-side eventually completed — see §8's "stuck server-side resource" failure mode).

**Parallel cycles.** Independent workdirs (different name prefixes, no shared cluster) can run apply + plan + destroy concurrently. Use this when you have several resources to verify and the wall-clock cost of cluster spin is the bottleneck.

**If reusing a cluster** — copy the minimal reuse stack:

```bash
cp .claude/skills/manual-test-redpanda-resource/templates/minimal_reuse_stack.tf.tmpl \
   manual-tests/<name>/main.tf
```

It declares `var.cluster_id`, looks up the cluster via `data.redpanda_cluster`, and has a placeholder `redpanda_<name>` block plumbing `cluster_api_url`. Pass the cluster id via `terraform.tfvars` (gitignored alongside). For resource-specific patterns beyond the placeholder (nested attributes, dataplane field shapes), look at `examples/<name>/main.tf` in the repo as the canonical reference.

**If creating fresh** — copy the minimal fresh stack:

```bash
cp .claude/skills/manual-test-redpanda-resource/templates/minimal_fresh_stack.tf.tmpl \
   manual-tests/<name>/main.tf
```

It creates a resource_group + network + dedicated AWS cluster (us-west-2) and has a placeholder for the resource under test. Override the cloud / region / cluster_type per §2 if your resource demands it.

### Wire in the local provider via dev_overrides

Build the binary at the repo root:

```bash
go build -o terraform-provider-redpanda
```

Drop in the dev_overrides config:

```bash
cp .claude/skills/manual-test-redpanda-resource/templates/dot_terraformrc.tmpl \
   manual-tests/<name>/.terraformrc
```

Run terraform with `TF_CLI_CONFIG_FILE=.terraformrc` from inside `manual-tests/<name>/`.

Critical:

- The dev override key is `hashicorp/redpanda`. Either omit `required_providers` (terraform defaults to that) or write `source = "hashicorp/redpanda"` explicitly. **Don't use `redpanda-data/redpanda`** as the source — terraform will silently fall back to the registry build and you'll spend an hour wondering why your code change doesn't show up.
- The dev_overrides value is a **directory path** (the directory containing the `terraform-provider-redpanda` binary), **not the binary path itself**. `"../../"` if your binary is at `<repo>/terraform-provider-redpanda` and you're running from `<repo>/manual-tests/<name>/`.
- dev_overrides bypass the lock file and registry checksums. `terraform init` is not strictly required; `rm -rf .terraform .terraform.lock.hcl` between runs to avoid stale state.
- Verify the override is active: `terraform plan` will print a warning like `Provider development overrides are in effect`. If you don't see that line, your dev binary is NOT being used.
- `task local:cluster:aws:apply` is the canonical reference invocation that sets all of this up.

Copy `templates/manual_test_results.md.tmpl` from this skill to `manual-tests/<name>/results.md` and fill it in as you go — it doubles as the run journal and the report you'll send back.

## 4.5 Datasource testing pattern (cluster-reuse)

Datasources are read-only — the test loop is much lighter than resources. Bundle every datasource test into a single workdir that targets a still-alive cluster (you'll usually have one from a resource cycle).

Pattern per datasource:

1. **Single-lookup datasources** (`cluster`, `network`, `resource_group`, `serverless_cluster`):
   ```hcl
   data "redpanda_cluster" "shared" { id = "<live-resource-id>" }
   output "cluster_api_url" { value = data.redpanda_cluster.shared.cluster_api_url }
   output "kafka_api"       { value = data.redpanda_cluster.shared.kafka_api }
   ```
   Verify outputs match what the resource created — especially nested computed objects (`kafka_api`, `schema_registry`, `http_proxy`, `aws_private_link.status`).

2. **List datasources** (`regions`, `serverless_regions`, `throughput_tiers`):
   ```hcl
   data "redpanda_throughput_tiers" "aws" { cloud_provider = "aws" }
   output "tier_names" { value = [for t in data.redpanda_throughput_tiers.aws.throughput_tiers : t.name] }
   ```
   Confirm non-empty list with the expected element shape. List datasources have no `id` filter — they're cheap to test in bulk.

3. **Plan-twice stability check**: after the apply, run `terraform plan -detailed-exitcode` twice. Both must return exit 0 (`No changes`). A first-plan-clean / second-plan-diff would expose unstable Read behavior in the datasource.

4. **No mutation / no destroy / no import** — datasources don't write state in any meaningful way.

A single workdir covering 7+ datasources runs in well under a minute against a live cluster. Worth doing alongside every cluster-resource cycle that the datasources cover.

### Datasource shapes to know

- `redpanda_region` takes `cloud_provider` + `name` and returns `zones`.
- `redpanda_regions` takes `cloud_provider` and returns `regions`.
- `redpanda_serverless_regions` returns `serverless_regions`, not `regions`.
- `redpanda_throughput_tiers` returns `throughput_tiers`.
- `redpanda_cluster` returns the full nested `kafka_api` block (seed brokers, mtls, sasl); assert on the nested leaves, not just the id.

**Datasource-schema-verify rule**: datasource field names diverge from their resource counterparts in ways that aren't obvious — `redpanda_region` takes `cloud_provider + name` (not `region` like the resource), `redpanda_regions` takes only `cloud_provider` (no `cluster_type`), `serverless_regions` exports `serverless_regions` (not `regions`). Always grep `docs/data-sources/*.md` for the actual field shape before writing the test workdir.

## 5. Test plan

Run each step. Record outcome and a relevant output snippet in `results.md` immediately, while context is fresh. If a step fails, fix the underlying code (not the test) and rerun from at least the affected step. Don't paper over a failing no-op plan by tweaking the test.

| # | Action | Pass criteria |
|---|--------|---------------|
| 1 | `terraform plan` (sanity check) | Plan resolves the provider via your dev_override. No registry-checksum errors. |
| 2 | `terraform apply` (initial create) | Resource creates without error. Final state matches schema. (Fresh: cluster lifecycle completes; reuse: only your new resource creates.) |
| 3 | **Feature validation** (resource-specific) | Hit the resource directly via `rpk`, gRPC, or the Cloud console — confirm the thing the resource is supposed to enable actually works. |
| 4 | `terraform plan` immediately after apply (no edits) | **Reports `No changes`.** Any unexpected diff = broken plan modifier or unstable Read. Stop and fix before continuing. |
| 5 | Update path: edit a mutable attribute → `terraform apply` | Update RPC fires (or RequiresReplace destroy/recreate, if expected). Plan + apply succeed. |
| 6 | `terraform plan` after update | `No changes`. |
| 7 | State remove: `terraform state rm redpanda_<name>.test` | Removed from state; resource still exists in cloud. |
| 8 | Re-import: `terraform import redpanda_<name>.test <id>[,<cluster_id>]` | Import succeeds. Subsequent `terraform plan` reports `No changes`. |
| 9 | Drift detection: mutate the resource out-of-band (rpk / console / API) → `terraform plan` | Plan detects the drift. (For some immutable attributes this means RequiresReplace.) |
| 10 | `terraform destroy` | Fresh: resource and cluster torn down. **Reuse: only the resource(s) you added are destroyed — the cluster stays up.** Verify in the Cloud console before declaring step 10 done. |
| 11 | Cleanup verification: `REDPANDA_CLOUD_ENVIRONMENT=pre task cleanup:redpanda:dry` | No stale `tfrp-*` clusters. If any, run `task cleanup:redpanda` and `task cleanup:aws:ci`. **Always set `REDPANDA_CLOUD_ENVIRONMENT=pre` for cleanup** (the taskfile defaults `REDPANDA_CLOUD_ENVIRONMENT` to `prod` for cleanup targets, which will fail auth with preprod creds). |

## 6. Upgrade scenario (required for extension or refactor branches)

Run this **before** the §5 test plan whenever the branch could change the on-disk state shape:

- Field additions (extension flow) — covers backwards-compat for existing user state.
- Refactor / migration branches that re-emit Flatten/Expand or move attributes between hand-written and generated code paths — covers schema-shape drift.
- Alias deprecations (canonical name added alongside a legacy one) — covers both alias paths via §6.5 below.

For these branches the §6 result is the **single most important check** in the entire run. Don't treat it as optional.

### Pre-stage two `.terraformrc` variants

```bash
# manual-tests/<name>/.terraformrc          — dev_overrides on hashicorp/redpanda (active)
# manual-tests/<name>/.terraformrc-released — released v1.X.0 plugin via filesystem_mirror, no dev_overrides
```

Switch between them with `TF_CLI_CONFIG_FILE=…`. Don't shuffle one file in/out — separate files lets you switch back during step 10's destroy without re-editing.

For the released-provider variant, install the binary from the GitHub release (not the registry — `terraform init` can fail to query platform metadata for older versions):

```bash
mkdir -p ~/.terraform.d/plugins/registry.terraform.io/redpanda-data/redpanda/<version>/darwin_arm64/
curl -sL -o /tmp/tfrp.zip \
  https://github.com/redpanda-data/terraform-provider-redpanda/releases/download/v<version>/terraform-provider-redpanda_<version>_darwin_arm64.zip
unzip -j /tmp/tfrp.zip 'terraform-provider-redpanda_v*' \
  -d ~/.terraform.d/plugins/registry.terraform.io/redpanda-data/redpanda/<version>/darwin_arm64/
```

Point `.terraformrc-released` at a `filesystem_mirror` for `redpanda-data/redpanda` (see `templates/dot_terraformrc-released.tmpl`).

### Steps

| # | Action | Pass criteria |
|---|--------|---------------|
| 0a | `main.tf` declares `source = "redpanda-data/redpanda"; version = "<released>"`. `TF_CLI_CONFIG_FILE=.terraformrc-released terraform init && terraform apply` a config that does NOT reference the new field. | Existing resource creates and stabilizes. Represents what users have today. |
| 0b | Edit `main.tf` source to `hashicorp/redpanda` (drop the version). **`terraform state replace-provider -auto-approve registry.terraform.io/redpanda-data/redpanda registry.terraform.io/hashicorp/redpanda`** (state was written by the released provider — without this, plan errors `Missing required provider`). `rm -rf .terraform .terraform.lock.hcl`. `TF_CLI_CONFIG_FILE=.terraformrc terraform plan -no-color -detailed-exitcode`. | **Reports `No changes` / exit 0.** Any diff means the new code broke backwards compat for existing state — stop and fix in the calling skill's code-review pass. |
| 0c | (extension only) Add the new field to your config, `terraform apply`. | New field round-trips; subsequent `terraform plan` reports `No changes`. |

Then proceed with §5. In `results.md`, prepend an "Upgrade scenario" section before the "Test plan" rows and tag the run as an **extension test** or **refactor test** in the Setup table so anyone reading the journal later knows the upgrade path was exercised.

### 6.5 Alias-deprecation testing

When the branch introduces a canonical attribute name alongside a deprecated alias (e.g. `tags`→`cloud_provider_tags`, `cloud_provider_config`→`aws_config`, `cluster_type`→`type`, `custom_properties_json`→`custom_properties`), exercise **both paths in separate cycles**:

1. **Legacy-alias path**: Phase 0a apply uses the deprecated name (matching existing user configs). Phase 0b must report `No changes` AND a second `terraform plan` immediately after must also report `No changes`. A perpetual `1 to change in-place` on the second plan = the canonical field is `optional+computed` without a `[UseStateForUnknown]` plan modifier; every plan recomputes and every apply fires a no-op Update RPC.
2. **Canonical-name path**: Phase 0a apply uses the canonical name. Phase 0b must report `No changes`. Then verify the deprecation warning fires when you flip back to the legacy name.

Both paths matter — a clean canonical path with a broken legacy path is the most common failure mode for deprecation aliases (the customer-facing pain is concentrated on existing users who haven't migrated yet).

## 6.6 Long-running steps: background + wakeup

Several steps exceed the Bash tool's 10-minute foreground cap:

| Op | Typical | Up to |
|---|---|---|
| Cluster apply (AWS dedicated tier-1) | 25 min | 55 min |
| Cluster destroy | 15 min | 30 min |
| Network apply | 4 min | 8 min |
| Network destroy | 3 min | 15 min |
| Serverless private link apply | 10 min | 15 min |
| PrivateLink reconfig (e.g. `allowed_principals` change) | 5 min | 20 min |

For any of these, use `Bash` with `run_in_background: true` and `ScheduleWakeup` for the expected completion window (slightly under, to leave buffer for the background-task notification). Don't tail-pipe (`| tail -N`) the output — it buffers and you see nothing until the whole command exits. Write to a log file with `tee` and inspect with a separate `tail` call.

Common mistake: starting an apply foreground with a long timeout, then losing the entire terminal session if the connection drops. Always background long applies.

## 6.7 Destroy-path fallback (when the dev binary can't refresh)

When the dev binary's plan / refresh errors before destroy can complete (e.g. a `Value Conversion Error` from a buggy generated Flatten, a `Missing required provider` after state-replace), the only way to free the live cluster is to flip back to the released provider:

1. Edit `main.tf` source back to `redpanda-data/redpanda` + the released version pin.
2. `terraform state replace-provider -auto-approve registry.terraform.io/hashicorp/redpanda registry.terraform.io/redpanda-data/redpanda`.
3. `rm -rf .terraform .terraform.lock.hcl` and `terraform init` with `TF_CLI_CONFIG_FILE=.terraformrc-released`.
4. `terraform destroy -auto-approve` (will use the released binary that can read its own state shape).

If destroy itself hangs more than expected, fall through to the §11 cleanup-task hammer.

Without this fallback you leak a $$ cluster — the dev binary cannot destroy state it cannot refresh.

## 6.8 Static-audit complement to live testing

Before spending 50 minutes on a live cluster, do a static-diff pass for the change class you're testing:

- **Hand-written vs generated AttrTypes** — when a refactor moves a resource from hand-written `Get*Type()` attribute-type helpers to schemagen-emitted `*AttrTypes()` functions, the two map definitions must match field-for-field. A 30-line Python brace-counting parser comparing the two maps catches divergence in seconds. Worth running before the cluster spin.
- **Schema vs proto field set** — `task generate` followed by `git diff` on `*_gen.go` files surfaces any `todo: true` / `exclude: true` mismatches between schema yaml and the actual proto shape.
- **`grep` for known footguns** — `id: RequiresReplace` (should be `UseStateForUnknown` for computed-only ids); `optional+computed` attrs without an explicit plan_modifier (perpetual diff loops); orphan helpers in `object_definitions.go` after a migration. Two leaves cannot be exercised from Terraform and should be marked as such rather than left as gaps: `topic.replica_assignments` (broker IDs are not exposed) and `acl.resource_type = DELEGATION_TOKEN` (not implemented server side).

Static signals are cheap and complementary to live tests. They catch field-set drift and shape regressions that live tests would only stumble onto when the specific field is non-null in state. For surgical fixes (a 1-line template addition, a one-key validator update), code review + targeted live re-verification of the specific failure mode is often enough — full upgrade-scenario re-spins are wasteful.

## 7. Dev-loop quirks

These bite repeatedly across iterations of the same test. Fix them once at the source rather than working around each occurrence.

- **Provider build is stale.** You changed code, ran the test, didn't see the change. Likely cause: you forgot to rebuild after editing. Always `go build -o terraform-provider-redpanda` from the repo root before each apply that's testing a code change. If you used `task build:install` instead, the cached binary at `~/.terraform.d/plugins/registry.terraform.io/hashicorp/redpanda/...` may be older than your repo build — prefer the dev_overrides path so there's only one binary to keep current.
- **Leftover state from a previous run.** `terraform.tfstate` in `manual-tests/<name>/` survives across test cycles. After step 10's destroy, `rm -f terraform.tfstate*` to start fresh next iteration. If you skip this and the previous run failed mid-apply, you'll get spurious "resource already exists" errors.
- **`.terraform/` cache mismatched with current binary.** After rebuilding, `rm -rf .terraform .terraform.lock.hcl` in the work dir before re-applying. The dev_overrides bypass the lock file but `.terraform/` can still hold a stale provider download from a prior `terraform init` that did consult the registry.
- **dev_overrides path is wrong.** If you set the value to the binary file (`"../../terraform-provider-redpanda"`) instead of the directory (`"../../"`), terraform silently falls back to the registry. The "Provider development overrides are in effect" warning is your tripwire — if it's missing, your override isn't loaded.
- **Mixing dev_overrides and `task build:install`.** Pick one. Running both can leave stale binaries in two places that fight each other depending on which `.terraformrc` terraform reads. The dev_overrides approach is the canonical recipe and what the rest of this skill assumes.
- **Required-providers source typo.** If `main.tf` has `required_providers { redpanda = { source = "redpanda-data/redpanda" } }`, the override (which keys on `hashicorp/redpanda`) won't match. Either remove the `required_providers` block or change the source string to `hashicorp/redpanda`.

## 8. Common failure modes

- **No-op plan shows a diff** → a Computed attribute is being unset by Read, a plan modifier is missing `UseStateForUnknown`, or a default isn't applied consistently between Create and Read.
- **Import leaves attributes empty** → `ImportState` doesn't seed enough state for `Read` to fully hydrate. Dataplane resources must seed `cluster_api_url`.
- **Destroy hangs or errors** → `allow_deletion=false` (canary). Either the resource didn't get the flip in the test config, or there's a real bug — don't flip the canary to silence it (see `CLAUDE.md`).
- **Cluster won't tear down** → kill terraform, then `REDPANDA_CLOUD_ENVIRONMENT=pre task cleanup:aws:ci` + `REDPANDA_CLOUD_ENVIRONMENT=pre task cleanup:redpanda`. The framework's destroy can't always recover.
- **`terraform init` fails with "provider redpanda-data/redpanda not found"** → wrong source string in `required_providers`. Use `hashicorp/redpanda` or omit the block entirely.
- **Auth error against `auth.prd.cloud.redpanda.com`** → preprod creds, prod env. Set `REDPANDA_CLOUD_ENVIRONMENT=pre`.
- **`Plan: 0 to add, 1 to change, 0 to destroy` immediately after apply** (with `-detailed-exitcode` returning 0) → an `optional+computed` attribute is missing `[UseStateForUnknown]`. The planner recomputes it to `(known after apply)` each cycle even though no real change is needed. Apply fires a no-op Update RPC every time. Fix in the schema yaml.
- **`Value Conversion Error: Path: timeouts` with `Received tftypes.Object[]`** → the generated Flatten dropped `prev.Timeouts`. Schemagen needs `m.Timeouts = prev.Timeouts` inside the `prev != nil` carry-over for any resource declaring a `timeouts:` block.
- **`Missing required provider redpanda-data/redpanda` after switching to dev_overrides** → you forgot the `terraform state replace-provider redpanda-data/redpanda hashicorp/redpanda` from §6 step 0b.
- **Stuck server-side resource (`STATE_IN_PROGRESS` deletion for hours/days)** → not a provider bug. The server's deletion task got wedged. Use a different name prefix for the next cycle (versioned workdir, §4) and let cleanup catch up eventually. Don't retry destroy in a tight loop — it won't unstick anything.

## 9. Report

After step 11, post a summary to the user covering:

- Cloud / cluster type / region used (and whether reuse or fresh)
- Pass/fail per step (call out the specific failure if any, with the exact error)
- Whether the upgrade scenario (§6) was run, and its result
- Any unexpected no-op-plan diff
- Cleanup confirmation (no stale resources)
- Path to the full `results.md`

The summary lives in chat; the file lives at `manual-tests/<name>/results.md` for follow-up.

For multi-cycle runs (re-test after a fix lands, sequence of refactor commits), maintain two files at the top of the workdir tree:

- **`manual-tests/SUMMARY.md`** — append-only per-cycle evidence trail (this is the journal). One entry per cycle with the check table, new findings, resources left alive, next-cycle gating. Don't rewrite history; older entries reflect state at write time.
- **`manual-tests/FINDINGS.md`** — always-current master table of every finding ever raised across cycles. Updated alongside SUMMARY.md whenever a cycle changes a status. Read FIRST to know what's currently open; SUMMARY.md is the deep dive for "what was the evidence." Structure: ID | Severity | Status | One-line | First seen | Resolved in, plus a "not provider-side" section for server/docs gaps.

Per-cycle SUMMARY.md entry structure:

```markdown
## Cycle YYYY-MM-DD (vNN) — <one-line theme>

### Scope
What this cycle was meant to verify.

### Setup
Provider binary state, cluster strategy, env, credentials path.

### Cycle log
| Check | Result |
|---|---|
| ... | ✅ live / ✅ code review / ⚠️ finding / ❌ blocker |

### Findings
NEW issues with severity, reproducer, fix sketch. Also update FINDINGS.md in the same change.

### Cross-cycle status diff
Which prior findings flipped status this cycle. NOT the full table — that's FINDINGS.md's job.

### Resources left alive
IDs + workdirs + intent for reuse (keep the cluster alive across cycles).

### Next-cycle gating
What the next cycle is blocked on, what's safe to proceed with.
```

When recording findings, distinguish:

- **✅ live**: verified end-to-end against a live cluster. Required for new behaviors, risky paths (Read/Flatten changes, plan modifiers on user-facing attrs), and anything where the upgrade-scenario assertion fires.
- **✅ code review**: read the diff, the change is surgical and semantically clear (one-line template addition, one-key validator update). Skipping a 50-min cluster respin is fine if the fix's semantics are obvious. Note `(code review)` explicitly so a future reader doesn't assume live coverage.
- **⚠️ finding**: real bug, blocks ship, has a known fix.
- **🟡 pre-existing / cosmetic**: bug that exists on `main` too or doesn't block a customer. Note for follow-up, don't gate ship on it.

This categorization is what lets a multi-cycle run produce a coherent "outstanding issues" list across days without re-litigating verified items.
