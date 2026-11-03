---
name: review
description: Review a PR or the current branch against .claude/review-policy.md with fanned-out finder agents, a per-candidate verify pass, and a gap sweep. Reports through ReportFindings; posts nothing.
argument-hint: "[pr-number] [low|medium|high|xhigh|max]"
allowed-tools: Agent, ReportFindings, Bash(gh pr view *), Bash(git rev-parse *), Bash(git log *), Bash(git diff *), Bash(git ls-files*), Bash(git fetch *), Bash(mkdir -p *)
---

Review $ARGUMENTS. The orchestration below mirrors the hosted multi-agent review, with this repo's angles added and execution allowed. You are the orchestrator; the `reviewer` agent does the finding and verifying.

## Phase 0: target and scope

1. Parse the arguments. A number is a PR; confirm it with `gh pr view <N> --json number,title,headRefName,headRefOid`, fetch the head, and refuse unless `git rev-parse HEAD` equals `headRefOid`: the diff, the file reads, and every verifier's `go test` run against the checkout, so a checkout on another commit reviews the wrong code (from `main`, the diff is empty and the review comes back clean). Tell the user to `gh pr checkout <N>` or open a worktree at the head and run `/review <N>` from there. A level word sets the effort; the default is `high`. `max` is the only level that spends Fable tokens, so it runs only when a person typed it as the argument, and it still pauses for a go before the first Fable spawn (Phase 1, below). When Claude invokes this skill on its own, for the pre-push rule or any other reason, the level is capped at `high` whatever was asked. `low`, `medium`, `high`, and `xhigh` never ask for permission. Without a number, review the current branch (`git rev-parse --abbrev-ref HEAD`) against `main` and refuse if it is `main`.
2. Scope is the whole change: `git diff main...HEAD` (or `origin/main` when the local main is stale) plus `git diff HEAD` for the working tree. A re-review after fixes is still the whole change. Record the diff size with `--numstat`.
3. Run the mechanical pre-check from the policy (secret-shaped paths in the changed files and in `git ls-files`). A hit becomes the first finding, by path only.
4. Create the scratch directory the agents will use for reproductions: the session scratchpad when one is listed, otherwise `manual-tests/review/<branch>/`. In the fallback, write a `go.mod` (`module review-scratch`, the repo's `go` line) into it before any agent spawns: `.gitignore` does not stop `go test ./...` from the repo root, and a nested module is the only thing that keeps a verifier's failing repro out of the maintainer's next `task test:unit`.

Tell the user in one line what is being reviewed and at what level.

## Phase 1: finders

Spawn one `reviewer` agent per angle, all in parallel, each with the same brief: the target, the diff commands, the scratch directory, the angle letter and name as the policy lists it, and a candidate cap. Do not summarise the diff for them and do not review alongside them.

Angles by level:

| Level | Angles | Cap per angle |
|---|---|---|
| `low` | A only, run by you inline on the hunks, no agents, no verify, at most 4 findings | 4 |
| `medium` | A, B, C, D, E, F, G, H, I, Conventions, Reuse, Simplification, Efficiency, Altitude, each only where it applies (below) | 6 |
| `high` | same set as medium, same applicability gate | 6 |
| `xhigh`, `max` | every angle, no applicability gate | 8 |

Applicability at `medium` and `high`, judged from the changed-file list and the diff, so that no agent is spent on an angle with nothing to examine: F runs when a `schema.yaml`, a `*_gen.go`, or a hand-written resource, datasource, action, or provider schema changes; G runs when anything under `internal/testutil/` or any `*_test.go` changes; H runs when the diff touches environment variables, credentials, HTTP or gRPC clients, retries, timeouts, or a cloud SDK; E runs when the diff adds or changes a wrapper, adapter, or pool around another type. A, B, C, D, I, Conventions, and the three cleanup angles always run.

Models and effort per role and level. Pass these on every spawn through the Agent tool's `model` and `effort` parameters; the agent file's own `model` is only the fallback for a spawn outside this skill.

| Level | Finders A, B, C, D, E | Finders F, G, H, I | Cleanup, Altitude, Conventions | Verifiers, correctness categories | Verifiers, cleanup categories | Gap sweep |
|---|---|---|---|---|---|---|
| `low` | none (inline) | none | none | none | none | none |
| `medium` | sonnet | sonnet | haiku | sonnet | haiku | none |
| `high` | sonnet | opus | haiku | opus | sonnet | none |
| `xhigh` | opus | opus | sonnet | opus, effort high | sonnet | opus |
| `max` | opus | opus | sonnet | fable, effort high (after the gate) | sonnet | fable (after the gate) |

Why this shape: finder cost scales with the number of angles times the size of the diff, so finders take the cheapest model that can carry the angle, and the repo angles F to I (where the historical misses were) step up a tier before the generic ones do. Verifier cost scales with the number of candidates but each verifier reads a narrow slice, so the strongest model goes there: a wrong verdict either ships a bug or wastes the author's time, and a verifier that runs code needs to design the reproduction correctly. Cleanup categories never justify more than the model that found them. Correctness categories are `correctness`, `schema-contract`, `fakes-fixtures`, `external-contract`, `stated-behaviour`, and `secrets`; cleanup categories are `conventions`, `reuse`, `simplification`, `efficiency`, and `altitude`.

Collect every candidate. Dedup only exact overlaps (same line, same mechanism), keeping the entry with the more concrete scenario. Keep everything else, including candidates you doubt.

Fable gate, `max` only. Finders at `max` run on opus and sonnet, so nothing above has cost Fable yet. Before Phase 2, tell the user in one short message: the diff size, the number of correctness-category candidates (each one is a Fable verifier at effort high), and that the Phase 3 sweep is one more Fable agent. Then stop and wait. Spawn nothing on Fable until the user says go in this session; a `max` argument typed earlier is the request, not the permission. If the answer is no, or the session is non-interactive, finish the review on the `xhigh` row (opus verifiers and sweep) and say so in the final report.

## Phase 2: verify

Spawn one `reviewer` agent per surviving candidate, in parallel, as verifiers, with the model the table gives for the candidate's category: give each the candidate, the diff, the files it names, and the scratch directory. Each returns CONFIRMED, PLAUSIBLE, or REFUTED with proof, per the policy.

- At `medium`, keep CONFIRMED and PLAUSIBLE where the verifier named a concrete trigger.
- At `high` and above, keep every CONFIRMED and PLAUSIBLE. One verdict other than REFUTED is enough to keep a finding.
- Drop REFUTED. A "new candidate" note from a verifier goes back through one verifier before it can be kept.

## Phase 3: gap sweep (`xhigh` and `max`)

Spawn one more `reviewer` finder, on the gap-sweep model from the table, with the verified list attached and a brief to look only for what is not on it: extracted or moved code that lost a guard, setup and teardown asymmetry in tests, a default flipped in config or in a fake, a docs promise with no code path, a control-plane change since the branch was cut. Cap 8. Route its candidates through Phase 2.

## Phase 4: report

Rank per the policy and call `ReportFindings` once with `level` and the findings. Then restate each finding in the reply on one line, `path:line, summary`, and name the level. No praise, no overview of the PR, no clean list. An empty review is an empty findings call plus one line saying so.

If the user asks to act on a finding, that is new work under the normal rules (test first, then fix), not part of this skill. When findings are fixed later in the session, call `ReportFindings` again with each finding's `outcome` set.
