---
name: implement-feature
description: Take a feature from context/features/, move it to "Doing", implement it on its own branch against its paired technical design, and verify the result with an independent subagent reviewer. Use when the user says "implement FEAT-XXX", "start on FEAT-XXX", "work on <feature name>", or "pick up the next feature".
---

# Implement Feature

Implements a single feature end-to-end on its own branch: locate it, mark it in
progress, read its paired technical design, build it following this repo's
workflow (`context/ai-interaction.md`) and coding standards
(`context/coding-standards.md`), then hand the diff to an independent subagent
for verification against the FEAT/TD docs before reporting back.

## Arguments

`$ARGUMENTS` names the feature — a `FEAT-NNN` id, a filename, or a title fragment
(e.g. `FEAT-000`, `core-identity-data-model`, "core identity"). If empty, ask the
user which feature to implement, or list `context/features/README.md` rows whose
Status is `Draft` and ask them to pick one.

## Steps

1. **Resolve the feature file.**
   - Find the matching file in `context/features/FEAT-NNN-*.md`. Confirm the match
     with the user if ambiguous.
   - Read it fully, and read its paired technical design (linked at the top under
     "Technical Design:", also at `context/technical-designs/TD-NNN-*.md`).
   - Check the feature's "Dependencies → Internal" section for `[[FEAT-XXX]]`
     references. If a dependency's status in `context/features/README.md` is not
     `Done`, tell the user and confirm whether to proceed anyway before continuing.

2. **Mark the feature as Doing.**
   - In the feature file itself, change the `Status: Draft` line (near the top)
     to `Status: Doing`. Update `Last Updated:` to today's date.
   - In `context/features/README.md`, update that feature's row in the Status
     column to `Doing`.
   - Do the same in `context/technical-designs/README.md` for the paired TD, and
     set `Status: Doing` inside the TD file's own header if present.
   - Populate `context/current-feature.md`: set the title, the `## File` link to
     the feature file, goals pulled from the feature's Functional Requirements
     (one goal checkbox per FR), and a short Notes summary.

3. **Create a branch.**
   - Per `context/ai-interaction.md`, create `feature/<short-slug>` off the
     current base branch before writing code. Confirm the branch name with the
     user if the feature title doesn't map cleanly to a slug.

4. **Implement against the technical design**, not just the feature doc — the TD
   is the source of truth for schema, components, API shape, and sequencing:
   - Follow the TD's "New Components" / "Modified Components" and "Data Model"
     sections exactly (table names, column types, constraints, indexes).
   - Respect migration ordering called out in the TD's "Dependencies" or
     "Sequence Flow" sections (e.g. TD-000 requires TD-002's `roles` table to
     exist before the `users` migration runs).
   - Follow `context/coding-standards.md`: Clean Architecture layering
     (handlers → services → repositories → domain models), interface-driven
     design with constructor-based dependency injection, explicit wrapped
     errors (`fmt.Errorf("context: %w", err)`), no global state, table-driven
     tests for every exported function.
   - Match existing project layout: `cmd/` entrypoints, `internal/` core logic,
     migrations via Goose, config in `config/`.
   - Implement every Acceptance Criteria checkbox under each FR in the feature
     doc — these define "done," not just the prose description.
   - Do not implement anything listed under the feature's "Non-Goals" section.
   - Make minimal, focused changes. Don't refactor unrelated code and don't add
     functionality beyond the FRs and their acceptance criteria.

5. **Test.**
   - Run `go build ./...` and `go vet ./...`; fix any errors before proceeding.
   - Run `go test ./...` for the affected packages. Add table-driven tests
     covering the feature's "Edge Cases" section.
   - If the feature touches an HTTP surface, verify it manually (e.g. `curl`)
     per `context/ai-interaction.md`'s workflow before declaring it done.

6. **Verify with an independent subagent before declaring anything done.**
   - Spawn an `Explore`-or-general-purpose subagent via the Agent tool (run it
     in the foreground — its findings gate the next step) with a self-contained
     prompt that:
     - Points it at the feature file and TD file paths (don't paraphrase their
       content — the subagent has no memory of this conversation, so tell it to
       read the docs itself).
     - Points it at what changed: the branch name and `git diff <base>...HEAD`
       (or the specific files touched), not a summary of your intentions.
     - Asks it to check, independently: every Acceptance Criteria checkbox
       against the actual code, the TD's Data Model/Components/API Design
       sections against what was built, the feature's Business Rules and Edge
       Cases against what's enforced, and Non-Goals against scope creep.
     - Asks it to run `go build ./...`, `go vet ./...`, and `go test ./...` and
       report failures.
     - Asks for a severity-ranked list back (Blocking / Should-fix / Note) with
       file:line evidence — not a pass/fail opinion.
   - This subagent step is the same verification method as the
     `review-technical-design` skill; reuse that skill's dimensions if invoked
     separately later, but here it runs automatically as part of implementation.
   - If the subagent reports Blocking findings, fix them and re-run the
     verification subagent before continuing. Don't self-certify — a fresh
     subagent with no stake in the implementation is the check, not your own
     read of the diff.

7. **Update checkboxes and status once the subagent verification is clean.**
   - Check off each satisfied Acceptance Criteria box in the feature file.
   - Check off each goal in `context/current-feature.md`.
   - Leave the feature's Status as `Doing` — do NOT set it to `Done` yourself.
     Report to the user what was implemented, the subagent's verification
     findings (including any Should-fix/Note items left open), and any Open
     Questions from the TD that still need a human decision. Let the user
     confirm completion and flip the status to `Done`/`Review` themselves.

8. **Do not commit or merge without explicit permission** — `context/ai-interaction.md`
   requires asking first, and commits must use conventional prefixes restricted to
   `feat:, fix:, chore:, build:, config:, style:` with no AI attribution footer.

## Notes

- If the technical design has unresolved "Open Questions" that block a design
  decision (e.g. TD-000's cascade-vs-restrict question), stop and ask the user
  before implementing that part rather than guessing.
- If something isn't working after 2-3 attempts, stop and explain rather than
  continuing to try random fixes, per `context/ai-interaction.md`.
