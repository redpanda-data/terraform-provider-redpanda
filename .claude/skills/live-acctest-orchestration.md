# live-acctest-orchestration

Drive a multi-target live acceptance test campaign against terraform-provider-redpanda preprod, with per-test event visibility, structured notifications, bug delegation, and cleanup discipline.

## When to invoke

User asks any of:
- "run the live-acc tests" / "kick off the full live-acc suite"
- "run cluster:aws + cluster:gcp" / any specific subset of `task test:*` targets
- "validate this branch end-to-end against preprod"

Do NOT invoke for:
- Unit tests (`task test:unit`) — no orchestration needed
- Single manual smoke tests — use the `manual-test-redpanda-resource` skill instead

## Show-your-work principles 

These are the load-bearing principles that emerged from real failures during that session. Internalize them before scripting anything.

1. **Plain `go test -v` only. No gotestsum.** Wrappers that compact output hide per-test failure messages. The session hit three back-to-back gotestsum issues (Go 1.25 build break → `--packages` CLI change → `pkgname` format hiding failures) before we ripped it out. Once we used plain go test, every bug was diagnosable from the log.

2. **No retries on flake.** `gotestsum --rerun-fails` and similar burn ~1h per retry provisioning another cluster. A real flake is signal worth investigating, not papering over. If the user wants flake tolerance, ask explicitly.

3. **`-count=1` always.** Go test caches results based on package + flags. A test that passed 30 seconds ago will be served from cache as instant-PASS with `(cached)` in the output. For live-acc that's catastrophically misleading.

4. **TF_LOG=WARN suppresses critical signals.** Apply/Read/Delete success doesn't emit at WARN level. A multi-minute silent period in the log might be normal cluster provisioning OR a hung process — you can't tell from the log alone. Use process-tree inspection (`pgrep -lf "terraform apply"`) and tfstate file mtime to verify liveness.

5. **The `allow_deletion=false` canary is intentional.** Cluster tests use it to fail-loudly when the destroy path is needed unexpectedly. When a test FAILs mid-apply, the destroy will be blocked and the cluster leaks. Track these leaks; do NOT flip the flag.

6. **Sweepers lie. Verify with `cleanup:redpanda:dry`.** The per-test `acc.Register` cleanup hooks can print "cluster not found" while the cluster is actually still on preprod. Don't trust the in-log cleanup message — always run `task cleanup:redpanda:dry` at the end of the campaign to see ground truth.

7. **Inline-fix toolchain / infra / cred issues. Delegate provider bugs.** Anything that blocks the whole queue (gotestsum version, expired creds, network down, taskfile config) you fix yourself because the user is waiting. Anything that affects one target (provider bug, test assertion mismatch) goes to a background bug-hunter agent so you can keep monitoring.

## Pre-flight

### 1. Confirm scope

The user almost always means "the live-acc test targets in `.tasks/test.yml`". Resolve any ambiguity:

- "all" → list every `task test:<name>` target (skipping `test:unit`), present them, get explicit approval
- "minus GCP BYOVPC" or similar exclusions → confirm the actual subset; flag if an exclusion doesn't match an existing target name
- Specific subset → confirm

**Do not expand scope unilaterally.** A session got classifier-denied for adding `shadowlink` to the queue without authorization. If a target is expensive (provisions multiple clusters, long timeout), call it out and let the user opt in.

### 2. Set up the run directory

```
.logs/acctest-<YYYY-MM-DD>/
├── .creds.sh         (gitignored — REDPANDA_* + cloud-provider creds)
├── runner.sh         (or just direct Bash run_in_background calls)
├── <target>.log      (one per target — full go test -v output)
├── events.log        (optional, if using runner.sh — Monitor reads this)
├── LEAKED_RESOURCES.txt
└── SUMMARY.log       (optional, end-of-run tally)
```

`.logs/` is in the repo's `.gitignore`. Writing creds anywhere is never safe. Ask the user for any necessary credentials

### 4. Notification

As the user their preferred notification method. Make sure to notify on the following areas
- Campaign start ("kicking off N targets in order: ...")
- Each target start (`acctest START <target>`)
- Each target completion (`acctest PASS/FAIL <target>` with elapsed time)
- Heartbeats every 15 min for long-running targets (with log-tail snippet)
- Bug discoveries (before spawning a bug-hunter agent)
- Cleanup-required notices at end

## Runner architecture

Two viable shapes — pick based on user preference and queue length.

### Shape A: Sequential runner.sh (best for full campaigns)

Single bash script iterates a `TARGETS=()` array, runs each via `task test:<target>` with output redirected to `.logs/<run>/<target>.log`, emits events to a shared `events.log` file that you tail via the `Monitor` tool. Heartbeats fire from a forked subshell.

Pros: single process to monitor, predictable serialization, sane resource usage.
Cons: long total wall-clock; one slow target blocks the rest.

### Shape B: Direct parallel Bash backgrounds (best for ad-hoc subsets)

For 2-4 targets, just launch each as a separate `Bash run_in_background: true` task. The harness auto-notifies on each completion.

Pros: parallel execution, no script needed, harness handles notifications.
Cons: no built-in heartbeats; you have to check process state on demand.

The 2026-05-27 session used Shape A for the initial 12-target full run, then switched to Shape B after the queue was paused mid-flight for bug investigation.

### Sample go test invocation (post-gotestsum)

In `.tasks/test.yml`, the per-target invocation should be:

```yaml
- |
  DEBUG=true \
  REDPANDA_CLIENT_ID="{{.REDPANDA_CLIENT_ID}}" \
  REDPANDA_CLIENT_SECRET="{{.REDPANDA_CLIENT_SECRET}}" \
  TF_ACC=true \
  TF_LOG={{.TF_LOG}} \
  VERSION={{.VERSION}} \
  {{.PATH_PREFIX}} go test \
    -count=1 -tags=live_test,<target> -v -timeout={{.TIMEOUT}} \
    ./redpanda/resources/<pkg>/...
```

Mandatory flags:
- `-count=1` — disable test cache
- `-tags=live_test,<target>` — the build-tag gates which test functions activate
- `-v` — per-test events
- `-timeout=<duration>` — `6h` for cluster tests, `1h` for control-plane-only

## Per-target lifecycle

For each target:

1. **Emit START event** (events.log + notification)
2. **Spawn `task test:<target>`** with stdout/stderr redirected to per-target log
3. **Fork heartbeat loop** (15-min interval): read log tail, log line count, elapsed; POST a notification
4. **Wait for exit**
5. **Kill heartbeat** when the task exits
6. **Parse outcome**:
   - exit 0 → PASS
   - exit non-zero → FAIL
   - Count `--- PASS:` / `--- FAIL:` lines for sub-test breakdown
7. **Emit PASS/FAIL event** with elapsed, pass/fail counts, last 5 log lines on FAIL
8. **On FAIL**: trigger the bug-handling subroutine (below)

## Long-running silence is normal — verify, don't assume hang

When a target's log goes silent for 10-30 min, that's typically:
- Cluster create polling (25-40 min on AWS/GCP dedicated)
- TF_LOG=WARN suppressing apply/destroy step logs

To distinguish "alive but silent" from "hung":

```bash
# Process liveness
pgrep -lf "<target>.test|terraform apply|terraform destroy"

# State file activity
ls -la /var/folders/*/T/plugintest*/work*/terraform.tfstate

# Inspect what terraform is provisioning
lsof -p <terraform-apply-pid> | grep cwd
# → /var/folders/.../plugintest<N>/work<M>/
grep '"type"\|"state"' /var/folders/.../plugintest<N>/work<M>/terraform.tfstate
```

A `terraform apply` subprocess running for 30+ min on a cluster create is healthy. A test binary alive with no terraform child for 30+ min is suspect. Use both signals.

## Bug-handling subroutine

When a target FAILs with a real provider/test bug (not toolchain/infra):

1. **Classify** — is this the same fingerprint as another already-failed target? If so, mention that — the user may want to wait for the in-flight agent to fix both at once rather than spawn a second agent.

2. **Extract the verbatim failure** — read the log, find the `--- FAIL:` line and surrounding context (10+ lines on each side). Include the test step number, the failing assertion, any `[ERROR]` SDK lines.

3. **Form a probable-cause theory** — one or two sentences. Reference the relevant file path if you can.

4. **Notify the user** with title `bug-hunter: SPAWNING <thing>` and body summarizing the bug.

5. **Spawn `general-purpose` Agent in background** (`run_in_background: true`) with a prompt that includes:
   - Verbatim failing log excerpt
   - Path to the per-target log file
   - Your probable-cause theory
   - Explicit instructions to POST a notification at THREE checkpoints:
     - `bug-hunter: ROOT CAUSE` — file:line + one-sentence cause
     - `bug-hunter: TESTS RED` — failing test names + assertions
     - `bug-hunter: FIX COMMITTED` — commit SHA + message
   - Instruction: "invoke the `resolve-redpanda-bug` skill"
   - Hard constraints: no push, no `//nolint`, no `.golden`/`.description` edits without authorization
   - Coordination guard: if their fix would overlap with another agent's territory, stop and ask

6. **Record the leaked cluster** (if `allow_deletion=false` blocked destroy) in `LEAKED_RESOURCES.txt`.

7. **Keep monitoring** the remaining queue while the agent works.

## Verify-and-re-run loop (after a bug-hunter agent completes)

The harness auto-emits a task-notification when the spawned Agent finishes. On that notification:

1. **Read the commit** — `git show <sha>` — verify the diff matches your theory and the agent's claimed root cause.
2. **Spot-check ripple** — did the agent touch only the file they claimed? Anything else in `git status` that shouldn't be there?
3. **Notify** that you've verified the fix and are re-running.
4. **Re-launch** the originally-failing target (give it a `_v<N>` suffix in the log filename so you don't overwrite the earlier evidence).
5. The harness will wake you on completion of the re-run too.

## Manual cleanup and state sweep

### Pre-flight: scan example fixture directories for stale terraform artifacts

`config.StaticDirectory()` in `terraform-plugin-testing` copies the entire fixture directory contents into a fresh plugintest workdir at **test launch** (not per-step) and the test uses that snapshot for every subsequent step. Two consequences:
- Leftovers in the fixture dir at launch time ride along into every step
- Mid-flight fixture edits are **NOT picked up** by an in-progress test — if you fix a fixture while a test is running, that test continues with the old fixture and you have to re-run it from scratch to validate the fix

If someone ran `terraform apply` manually against an example fixture (local validation, dev-override testing, debugging), the leftovers will ride along and break the test. Worst offender is a stale `terraform.tfstate` whose recorded resource IDs belong to a different account — terraform refreshes against the bad state on Step 1, the API returns `PermissionDenied`, and the test fails in seconds with a UUID the test never generated.

All of these are typically gitignored so `git status` won't surface them. Scan with:

```bash
find examples -maxdepth 3 \( -name terraform.tfstate -o -name 'terraform.tfstate.backup' \
  -o -name '.terraform' -o -name '.terraform.lock.hcl' -o -name '.terraform.d' \)
```

If anything turns up, ask the user before removing — these are local artifacts that may belong to an in-progress manual investigation. After confirmation, `rm -rf` the matches and re-run the failing target.

### Manual cleanup when sweepers fall short

The taskfile sweepers (`cleanup:redpanda`, `cleanup:aws:ci`, `cleanup:gcp:ci`) and the per-test `acc.Register` hooks both have failure modes that leave real resources behind:

- **Sweeper false negatives** — in-test hook prints "cluster not found" while the cluster is still on the control plane. Always cross-check with the `:dry` variant before declaring a clean campaign.
- **Async-deletion races** — networks/clusters delete asynchronously. A resource group can't be deleted while a dependent network is still in `DELETING`. Re-run `cleanup:redpanda` two or three times spaced ~2-3 min apart.
- **UUID validation errors** in the sweep path — symptom of a sweeper passing a name where the API expects a UUID. The cluster sweeper may succeed while the network or resource group sweeper aborts; record what's left for the manual cleanup pass.
- **Cross-account `PermissionDenied`** — service account can create but can't read a resource whose UUID belongs to another account. Surfaces stale checked-in state; resolve via the pre-flight scan above.

For every leak the in-test sweeper missed, record name + cloud + region + likely cause in `LEAKED_RESOURCES.txt`. Reconcile against the post-campaign cleanup pass.

## Cleanup discipline

At the END of the campaign (whether all PASS or some FAIL):

1. **Stop any in-flight runners / monitors** cleanly (SIGTERM the runner; the test subprocess can continue if mid-destroy).
2. **Run `task cleanup:redpanda:dry`** — gives ground truth on what's still on preprod. Cross-reference against `LEAKED_RESOURCES.txt`.
3. **Run `task cleanup:redpanda`** if there are leaks. Expect partial success on first run: networks delete async, so resource groups may fail with "not empty" on first pass. Re-run after a few minutes for the second pass.
4. **AWS BYOVPC infra** (if any byovpc:aws ran): `task cleanup:aws:ci`.
5. **Final notification** with the campaign result summary.

## Common gotchas (each one bit us during  session)

| Gotcha | Symptom | Fix |
|---|---|---|
| `gotestsum` Go-toolchain mismatch | every target FAIL in <1s with `tokeninternal.go:64 invalid array length` | drop gotestsum, use plain `go test` |
| `gotestsum --rerun-fails` flake-retry | one failed target burns 2-3 hours retrying | drop `--rerun-fails`, accept FAIL as signal |
| go test cache `(cached)` PASS | target "passes" in 0s with no work done | add `-count=1` to all invocations |
| `gotestsum --format=pkgname` hides failures | `1 failure` but no test name visible | use `go test -v` directly |
| TF_LOG=WARN silent during apply | log unchanged for 30+ min during cluster create | verify via `pgrep` + tfstate mtime |
| `allow_deletion=false` canary | test FAIL leaks the cluster | track in LEAKED_RESOURCES; user discipline says don't flip the flag |
| Stale `terraform.tfstate` in fixture dir | target FAILs in 2-3s with `PermissionDenied : unable to request resource group with ID <UUID>` where UUID is NOT one the test just created — same UUID across re-runs | someone ran `terraform apply` manually against the example fixture; the leftover `terraform.tfstate`/`.terraform/`/`.terraform.lock.hcl` get copied by `config.StaticDirectory` into the plugintest workdir. Delete the local-only artifacts (gitignored, so deletion is purely local). Check `ls -la examples/<scope>/<provider>/` for any of those files |
| Sweeper "cluster not found" but cluster exists | misleading cleanup log | verify with `cleanup:redpanda:dry` |
| RG delete "FailedPrecondition: not empty" | RG cleanup partial-success | networks delete async; re-run cleanup after 2-3 min |
| Build-tag shared infra bugs | one bug fails multiple targets | recognize the shared fingerprint; coordinate with the bug-hunter agent |
| Test fixtures vs assertion helpers drift | fixture updated, but `BuildTestCheckFuncs` not | grep `internal/testutil/acc/checks.go` for the assertion site when changing fixture conventions |

## What this skill is NOT

- A bug-fixer — that's `resolve-redpanda-bug`'s job, invoked via the bug-handling subroutine above
- A new-resource scaffolder — use `add-redpanda-resource`
- A unit-test runner — use `task test:unit` directly, no orchestration needed
- A push automaton — never push without explicit per-push approval

## Artifacts a campaign should produce

- One log file per target under `.logs/acctest-<date>/`
- `LEAKED_RESOURCES.txt` listing every cluster/network/RG that needs follow-up sweep
- `SUMMARY.log` (optional) — chronological PASS/FAIL/elapsed/error-tail tally
notifications matching the protocol above
- (If bugs found) commit SHAs from the spawned bug-hunter agents
- A final user-facing summary with: targets PASS, targets FAIL, bugs found and fixed, leaks remaining, follow-up items