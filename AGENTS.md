# AGENTS.md

Repository working instructions for future Codex sessions.

## Workflow

- Do each coherent chunk of work in its own git branch.
- Use clear branch names:
  - `feature/<short-description>`
  - `fix/<short-description>`
  - `chore/<short-description>`
- Keep changes scoped and reviewable.
- When a chunk is complete, merge it back to `main`.
- Push `main` after merge.
- Pushing `main` triggers the GitHub Docker build used for testing.
- Do not start a new chunk of work until the previous branch is merged and pushed, unless explicitly asked.
- Before starting implementation, show the current git status.

## Source Of Truth

- Do not invent behavior, schemas, APIs, tracker types, config fields, endpoints, or assumptions.
- If a required source of truth is missing or unclear, stop and ask where to get it.
- Prefer repository code, checked-in docs, upstream source files, official API docs, and real examples over guesses.
- If evidence is weak, say so clearly.
- For external integrations, prefer real upstream schema/API output or official documentation over inferred behavior.

## Planning

- Before implementing, summarize the intended change.
- Identify files likely to be touched.
- Ask questions before changing architecture, data models, external integrations, migrations, or user-facing behavior.
- For larger work, propose phases and wait for approval.
- Keep plans tied to the current project priorities and confirmed behavior.

## Implementation

- Make minimal, targeted changes.
- Follow existing project style and patterns.
- Avoid broad rewrites unless explicitly requested.
- Preserve existing behavior unless the task requires changing it.
- Add comments only where they clarify non-obvious logic.
- Keep core PTV tracker config separate from integration-specific config.
- Do not remove stale integration settings unless a current valid schema is available and the save path intentionally normalizes them.
- If schema drift prevents safe operation, preserve existing data and ask for/rely on re-import or an explicit source of truth.

## Testing

- Run relevant tests or checks when available.
- If tests cannot be run, explain why.
- Report exactly what was tested and what was not.
- Prefer focused tests for risky merge, schema, secret-preservation, config, and sync logic.
- Do not claim behavior is verified unless it was actually exercised.

## Git Hygiene

- Show git status before and after work.
- Review diffs before committing.
- Use clear commit messages.
- Never commit secrets, local config, logs, build artifacts, or unrelated formatting churn.
- Never revert unrelated changes unless explicitly requested.
- Do not use destructive git commands unless explicitly requested and confirmed.

## Communication

- Be direct about uncertainty.
- Ask focused questions when blocked.
- Do not claim something is done unless it was actually changed and verified.
- When handing work back, include:
  - branch name
  - commit hash
  - summary of changes
  - tests/checks run
  - anything that should be verified in the Docker build
