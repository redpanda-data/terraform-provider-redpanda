---
name: review
description: Review a PR or the current branch against .claude/review-policy.md using the read-only reviewer agent. Prints findings; posts nothing.
argument-hint: "[pr-number]"
disable-model-invocation: true
allowed-tools: Agent, Bash(gh pr view *), Bash(git rev-parse *), Bash(git log *)
---

Review $ARGUMENTS. If no PR number was given, review the current branch against `main`.

1. Resolve the target. With a number, confirm it exists via `gh pr view <N> --json number,title,headRefName`. Without one, note the branch from `git rev-parse --abbrev-ref HEAD` and refuse if it is `main`.
2. Spawn the `reviewer` agent with the target and nothing else. Do not summarize the diff for it and do not review in parallel yourself.
3. Print the agent's report verbatim. Do not soften findings, add praise, or post anything to GitHub.
4. If the user asks to act on a finding, that is new work under the normal rules (test first, then fix); it is not part of this skill.
