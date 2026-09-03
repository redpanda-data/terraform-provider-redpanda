---
name: writing-code-comments
description: >
  Gates whether a Go comment should exist and forces the ones that stay to explain why, not what.
  Use ALWAYS before writing or editing a comment in Go, YAML schema files, or Terraform examples, and when reviewing a diff that adds or changes comments.
  Removes the comment types that clutter this repo: narration that restates the code, change history and session context ("previously did X", "per PR #123", "now that the fake ..."), ticket and PR numbers, perishable measurements, commented-out code, and doc comments that repeat the identifier.
  Keeps the ones that earn their place: a non-obvious why, a warning about a non-local consequence, a pointer to context a future reader can't reconstruct.
  Not for commit messages, PR descriptions, or schema `description:` strings (those are proto-sourced).
---

# Writing code comments

Run this before adding or editing any comment. The default is no comment. Clear names carry most of the meaning; a comment earns its place only when it tells a reader something the code cannot.

## The gate: one question

> **What does this tell a future reader that the code itself doesn't?**

If the answer is "it restates what the code does", delete it. Rename the identifier or extract a function instead.

A comment worth keeping answers a why the code can't:

- ✅ `// The control plane echoes an empty list as null; normalize so the plan stays empty.`
- ✅ `// UseStateForUnknown must precede RequiresReplace: the framework nulls unknowns first and RR would arm a replace every plan.`
- ✅ `// Kept in sync with the enum carve-outs in codegen.yaml; change both.`

## Delete these

### Narration that restates the code

- ❌ `// increment the counter` above `counter++`
- ❌ `// loop over listeners` above `for _, l := range listeners`
- ❌ `// return the diagnostics` above `return diags`

If a block needs narration to be followed, the fix is smaller functions and better names.

### Change history and session context

Never record how the code got here. That belongs in the commit message, attached to the diff and searchable. In source it goes stale immediately and misleads the next reader (and the next agent) about what the code does today.

- ❌ `// previously used a map here, switched to a slice for ordering`
- ❌ `// per PR #358` / `// as discussed` / `// ENG-1234`
- ❌ `// the fake now omits ServiceAccount, so ...` (describe what the fake does, not that it changed)
- ❌ `// this used to sail through every plan gate` (describe the invariant being pinned)
- ❌ `// Claude: added this helper` / `// AI-generated`
- ❌ `// TODO(2026-03): remove after migration` left in after the migration

The tell is a verb of change aimed at the past: previously, now, no longer, used to, was added, switched to. Rewrite in the present tense stating the current rule. `scripts/lint-comments.sh` fails the build on the unambiguous markers (ticket IDs, PR refs, review chatter, session context); the rest is caught here and at review.

### Perishable measurements and current-state stamps

Timings, counts, and rates rot silently. State the durable relationship instead.

- ❌ `// cluster create takes ~40 min` when the durable fact is that create is slow and the timeout is generous
- ❌ `// no resource currently opts into this` where dropping "currently" states the same fact

Numbers that stay: a dated snapshot (`// as of 2026-08, the control plane caps this at 100`), a restated adjacent literal, a platform constant, a target or budget, a cited upstream issue that dates itself.

### Commented-out code

Delete it. Git has it. The next reader can't tell whether it's a note, a rollback plan, or an accident.

### Doc comments that repeat the identifier

- ❌ `// GetUserByID gets the user by id.`
- ❌ `// setConfig sets the config.`

golangci's `revive` wants exported identifiers documented. Satisfy it with the one fact the name doesn't carry: the invariant, the failure mode, the unit, the side effect. If there is no such fact, the identifier is either well named (one short line is fine) or badly named (fix that).

## Keep these

- A **why** that isn't obvious: a control-plane quirk, a framework ordering constraint, a proto field that lies about its writability, a retry classifier decision.
- A **warning** about a consequence that lives elsewhere: "the golden pins this", "changing this breaks the upgrade entry", "the fake mirrors this on Update too".
- A **pointer** to context a reader can't reconstruct: an upstream issue (`terraform-plugin-framework#1211`), a proto path in `../cloudv2`, the doc page a behavior is specified in. Our own ticket numbers are not this; the ticket won't be readable in a year.

## Style

- **Explain why, not what.** The what is in the code.
- **Present tense, active voice, one idea per sentence.** State the cause and the effect. Name the actual field, value, or condition.
- **Length follows content.** A one-liner is fine when one line covers it. A block over about ten lines is a signal the explanation belongs in a test name, a doc file, or the commit message; `scripts/lint-comments.sh --report` lists such blocks.
- **No em-dash.** Use a connective: because, so that, which means, to avoid.
- **Preserve existing comments when moving code** unless the move makes them wrong. Don't drop a why because you relocated the function.
- **Match the surrounding density.** Don't annotate every line of a bare file; don't strip a well-commented package.
- **Test files are not exempt.** A test comment states the invariant under test or the input shape, never the bug's history. The test name plus a one-line "pins X" is the ceiling.

## When you're tempted to comment

Try, in order: (1) a better name, (2) a smaller function, (3) a type or a named constant, (4) a test whose name states the rule. Reach for a comment only when none of those can carry the meaning.
