---
name: review-technical-design
description: AI reviewer that checks whether a feature's technical design was implemented correctly — compares TD/FEAT docs against the actual code. Use when the user says "review FEAT-XXX", "check TD-XXX was implemented correctly", "audit the implementation of <feature>", or asks whether a feature is really done.
---

# Review Technical Design

Independent reviewer role: verify that code in the repository actually satisfies
a feature's technical design (TD) and its acceptance criteria — not just that
code was written. This is a check, not an implementation task; do not fix code
unless the user explicitly asks you to after seeing the findings.

## Arguments

`$ARGUMENTS` names the feature to review — a `FEAT-NNN`/`TD-NNN` id or title
fragment. If empty, ask which feature to review, or offer features whose Status
is `Doing` in `context/features/README.md` as candidates.

## Steps

1. **Load the source of truth.**
   - Read the feature file (`context/features/FEAT-NNN-*.md`) in full: Functional
     Requirements, Acceptance Criteria, Business Rules, Edge Cases, Non-Goals.
   - Read the paired technical design (`context/technical-designs/TD-NNN-*.md`)
     in full: Data Model, Components, API Design, Sequence Flow, Security,
     Risks, Open Questions.
   - Note any `[[FEAT-XXX]]` cross-references that this feature depends on or
     that depend on it — implementation correctness may hinge on those contracts.

2. **Locate the implementation.** Use `git log`/`git diff` against the base
   branch (or the files the user points to) to find what was actually changed
   for this feature. Read the real code — migrations, models, handlers,
   services, repositories, tests — don't infer correctness from commit messages
   or the feature doc's checkboxes.

3. **Verify systematically, one dimension at a time:**
   - **Data model fidelity**: for each table/column/constraint/index in the TD's
     "Data Model" section, confirm the migration matches (types, nullability,
     defaults, FK actions like `ON DELETE CASCADE`, unique constraints, named
     indexes). Flag drift in either direction — missing pieces and undocumented
     extras.
   - **Component completeness**: every item under "New Components" /
     "Modified Components" exists and does what the TD says.
   - **API contract**: for each endpoint in "API Design", confirm route, method,
     request/response shape, and status codes match what's implemented.
   - **Acceptance criteria**: walk every checkbox under every FR in the feature
     doc; mark it verified-true, verified-false, or unverifiable-from-code (e.g.
     needs a running system to check).
   - **Business rules & edge cases**: check that each rule/edge case listed in
     the feature doc is actually enforced in code (constraint, validation, or
     test), not just plausible by omission.
   - **Non-goals respected**: flag any implemented functionality that the
     feature doc explicitly marked out of scope — scope creep is a finding too.
   - **Security & risk mitigations**: check the TD's "Security" and "Risks →
     Mitigation" sections were actually built in (e.g. `password_hash` tagged
     `json:"-"`, tenant-scoping on queries, mitigations for named risks).
   - **Coding standards & Go best practices**: read `context/coding-standards.md`
     in full and check every changed file against it, not just a spot-check —
     note violations but weight them below functional correctness:
     - **Architecture**: Clean Architecture layering respected (handlers only
       parse/validate/call-service/respond; services hold business rules with
       no HTTP concerns; repositories only persist, no business logic);
       package-by-domain, not package-by-layer; no new/expanded `models`,
       `utils`, `helpers`, or `common` packages.
     - **Errors**: every error checked and handled explicitly; wrapped with
       `fmt.Errorf("context: %w", err)` at boundary crossings; callers branch
       with `errors.Is`/`errors.As`; sentinel/custom errors defined in the
       owning package, not a global errors package; no `panic` outside
       `Must`-prefixed startup functions; no swallowed (`_ = err`) errors on a
       path that can actually fail.
     - **Interfaces & DI**: public functions accept interfaces, not concrete
       types, where a consumer needs substitutability; interfaces declared next
       to the consumer, sized to only the methods it calls; dependencies wired
       via constructor injection — no service locators, no package-level
       mutable globals.
     - **Concurrency**: any goroutine has a clear owner/lifecycle and exits
       cleanly on `context.Context` cancellation; shared mutable state guarded
       by a mutex or channel; no unbounded goroutine spawning per-request
       without a cap/pool.
     - **Context propagation**: `context.Context` is the first parameter and is
       actually threaded through service/repository/external calls, not
       dropped or replaced with `context.Background()` mid-chain.
     - **Resource handling**: `defer` used to close rows/files/connections/tx
       right after acquisition; no leaked handles on early-return error paths.
     - **Idiomatic Go**: short, single-responsibility functions; naming follows
       Go conventions (MixedCaps, no stutter like `user.UserID`); no
       unnecessary allocations or premature optimization; `net/http.ServeMux`
       method+pattern routing per Go 1.22+ conventions; exported
       functions/types have GoDoc-style comments.
     - **Tooling**: `gofmt -l .` / `goimports -l .` clean, `go vet ./...`
       clean, and `golangci-lint run` clean if the repo has it configured —
       run these, don't assume.
     - **Security-adjacent Go patterns**: no plaintext secrets in structs sent
       over the wire (`json:"-"` on sensitive fields), input validation before
       a service touches raw request data, no SQL built via string
       concatenation (parameterized queries only).
   - **Tests**: confirm tests exist for the acceptance criteria and edge cases,
     and that `go build ./...`, `go vet ./...`, and `go test ./...` pass. Run
     them.

4. **Classify every finding by severity** before reporting:
   - **Blocking**: acceptance criterion unmet, data model mismatch that would
     break a downstream feature, security/tenant-isolation gap, build/test
     failure.
   - **Should-fix**: business rule or edge case not enforced, missing test
     coverage for a specified scenario, coding-standard violation with real
     consequences (unwrapped errors swallowing context, global state).
   - **Note**: minor deviations, open questions from the TD still unresolved,
     scope creep that's harmless but undocumented.

5. **Report findings to the user directly** (in conversation) as a structured
   list grouped by severity, each with: what the TD/FEAT required, what the code
   actually does, and the file/line evidence. Do not silently patch issues.

6. **Only after reporting**, if the user asks you to fix findings, do so as a
   separate step — re-verify afterward rather than assuming the fix is correct.

7. **If everything blocking and should-fix is resolved**, offer to close out the
   feature — but only with the user's confirmation, and only after re-running
   build/vet/test to confirm the final state is green. Closing out means all of:
   - Set Status to `Done` in `context/features/README.md`'s row for this
     feature, and in the `Status:` line inside the feature file itself. Update
     the feature file's `Last Updated:` to today's date.
   - Set Status to `Done` in `context/technical-designs/README.md`'s row for
     the paired TD, and in the TD file's own header `Status:` line if present.
     Update its `Last Updated:` too.
   - Reset `context/current-feature.md` back to its placeholder state (below)
     — it tracks the single feature currently in progress, so once this one is
     `Done` it must not keep pointing at it.

   Placeholder content for `context/current-feature.md`:

   ```markdown
   # Current Feature

   _No feature currently in progress._

   ## File

   _(link to context/features/FEAT-NNN-*.md once one is picked up)_

   ## Goals

   _(populated from the feature's Functional Requirements when work starts)_

   ## Notes

   _(short summary populated when work starts)_
   ```

## Notes

- This is an adversarial, independent check — don't trust the feature file's
  own checkboxes or a prior session's claim that something is "done." Verify
  against the actual code and actual test runs.
- If the TD itself has unresolved "Open Questions," don't treat the
  implementation's choice on that question as automatically correct — surface
  it as a finding so the user can confirm the decision was intentional.
- Never commit, push, or change Status to `Done` without explicit user
  confirmation, per `context/ai-interaction.md`.
- Only reset `context/current-feature.md` if its `## File` link actually points
  at the feature being closed out — if it's already tracking a different
  feature (someone moved on before this review ran), leave it alone and just
  flag the mismatch to the user.
