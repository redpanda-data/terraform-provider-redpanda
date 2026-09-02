# Review policy

Judgment policy for code review in this repo, whether run interactively (`/code-review`, the `code-reviewer` agent) or by a CI reviewer. The rules a change is judged against live in `CLAUDE.md` and the skills; this file says only how to review: what clears the bar, what to filter, how to report.

## Signal bar

Flag only high-signal issues:

- A clear `CLAUDE.md` or skill violation where you can quote the rule broken: hand-edited `*_gen.go`, a modified golden without approval, a `//nolint`, a deleted or skipped test, a fixture modeling unreachable state, a lenient fake change.
- Schema-contract mistakes: a `oneof` arm marked `Computed`, `RequiresReplace` missing on a create-only field or present on an updatable one, a server-default field that isn't Optional+Computed, a state modifier ordered after `RequiresReplace`.
- Bugs and security issues: nil dereferences, swallowed diagnostics, retries that mask errors, drift-invisible flatten paths.
- Secrets: a credential-shaped literal anywhere in code, fixtures, examples, or logs, or a tracked secret-shaped file (`.env*`, `*.pem`, `*.key`, `*.p12`, `*.tfvars`, anything under `secrets/` or `credentials/`, credential or service-account JSON), templates ending in `.example`, `.sample`, or `.tmpl` excepted. Report the path and line, never the value. This is always the first finding.
- Test-tier mismatch: a behavior that could be a unit or integration test landing as live acc, or a live-only behavior with no acc coverage.
- Commit-shape violations: regenerated output mixed into a hand-written commit, ticket or PR numbers in messages.

If you are not certain an issue is real, do not flag it. Verify against the proto in `../cloudv2` or `../console` before flagging a writability claim.

## Filter out

- Pre-existing issues on lines the author did not modify
- Anything the linter, compiler, or `task docs` already catches
- Stylistic preferences not written down in `CLAUDE.md`
- Intentional behavior changes described in the PR
- Pedantry a senior maintainer would not raise

## Comments in the diff

Flag a new or edited comment when it:

- restates the code beneath it
- records process: ticket numbers, PR numbers, "per discussion", "previously", "now", session context
- states a perishable measurement without a date
- no longer matches the code it annotates

## Report format

Most severe first. For each finding: file and line, one-sentence defect, the concrete failure scenario. Say "LGTM" when nothing clears the bar.
