# Repository instructions

## General principles

- Preserve existing working behavior unless the user explicitly requests a functional change.
- Prefer focused, maintainable changes over broad rewrites.
- Keep the scanner, import pipeline, game data, optimization engine, and frontend concerns clearly separated.
- Reuse shared models and utilities when appropriate, but do not couple otherwise independent application layers.
- Remove obsolete code only after verifying that it is no longer used.
- Do not introduce compatibility layers, fallback mappings, or abstractions without a concrete current need.

## Language and localization

- Write all source-code comments, technical documentation, commit messages, and developer-facing text in English.
- Every user-visible frontend or backend message must use the localization system.
- Do not hardcode English, French, or other user-visible labels directly in React or Go code.
- Keep language-specific content inside the appropriate localization catalogs.
- English identifiers are the canonical keys whenever a stable textual identifier is required.

## Project data

- Treat the project data under `data/game` as the source of truth for game names, labels, statistics, sets, Arcs, characters, resources, and combat information.
- Do not compensate for missing or incorrect project data with hardcoded mappings in Go or frontend code.
- If required data is missing, report the exact missing file, record, field, or contract instead of creating a repository-specific exception.
- Keep presentation translations separate from game data. Presentation catalogs should only contain application-interface text and intentional display metadata.
- Remove unused data fields only after verifying their consumers across Go, frontend code, tests, documentation, and build scripts.
- When changing a JSON schema, update its consumers and tests together. Preserve backward compatibility when reasonable; otherwise document the migration clearly.

## Architecture

- Keep domain logic out of React components whenever it can be expressed and tested in Go or a dedicated frontend utility.
- Keep filesystem, scanner, packet decoding, import, scoring, optimization, damage calculation, localization, and UI concerns in their respective packages.
- Avoid duplicated business rules between the frontend and backend.
- Prefer explicit typed contracts between application layers.
- Keep generated files, runtime files, user data, and source-controlled project data clearly separated.
- Do not introduce dependencies between the scanner and optimizer that prevent either subsystem from being understood or tested independently.

## Optimization and scoring

- Treat scoring and search behavior as sensitive domain logic.
- Do not modify a ranking, pruning, candidate-selection, constraint, stat, set, or damage formula without adding or updating focused tests.
- Document formulas and non-obvious heuristics in English close to their implementation or in the relevant technical documentation.
- Ensure that the same statistic or bonus is not counted more than once across final stats, equipment relevance, objective fit, set activation, and damage calculations.
- Clearly distinguish strict feasibility constraints from ranking preferences.
- Keep displayed ranking values consistent with the formula actually used to sort results.
- Preserve the currently equipped build as a searchable candidate when the selected search mode permits approximate pruning.
- Mark approximate results honestly and never claim confirmed optimality unless the search can prove it.

## Frontend

- Keep components focused and extract shared behavior when duplication becomes meaningful.
- Use the established UI components and design tokens instead of introducing isolated styling conventions.
- Keep layouts responsive and verify dense tables, dialogs, cards, filters, and equipment views at practical desktop widths.
- Every new user-visible label must be added to every supported localization catalog.
- Do not expose internal identifiers when a localized display label exists.
- Keep detailed technical explanations in dedicated dialogs or expandable sections instead of overloading the primary workflow.
- Preserve accessibility semantics for dialogs, buttons, tabs, tables, labels, and status messages.

## Scanner, imports, and user data

- Treat packet captures, decoded exports, inventories, profiles, logs, temporary files, and generated workspace contents as user or runtime data.
- Never commit captures, imported account data, generated JSON exports, executables, archives, logs, or temporary files unless they are intentional sanitized test fixtures.
- Keep runtime data outside the repository whenever the application supports it.
- Clean temporary scanner input and output according to the application lifecycle.
- Do not expose account identifiers, local paths, packet contents, or other user-specific information in documentation, fixtures, logs, or commits.
- Maintain passive capture and decoding behavior; do not introduce gameplay automation or network modification unless explicitly requested and reviewed separately.

## Code quality

- Follow standard Go conventions, including clear package boundaries, small focused functions, explicit error handling, and `gofmt`.
- Avoid global mutable state, hidden side effects, premature abstractions, and duplicated validation.
- Prefer typed structures over unstructured maps when the schema is stable.
- Keep React state minimal and derived values memoized only when it provides a measurable clarity or performance benefit.
- Delete obsolete code, assets, scripts, and documentation only after checking all references.
- Do not silently swallow errors. Return, wrap, log, or present them at the appropriate application boundary.
- Add regression tests for every confirmed bug when practical.

## Validation

- After significant Go changes, run:

  ```powershell
  go test ./...
  ```

- After significant frontend changes, run the frontend tests and production build:

  ```powershell
  npm test -- --run
  npm run build
  ```

- Run the complete application build before a release.
- Use `git diff --check` before committing.
- If a validation step cannot run, report exactly which step was skipped and why.
- Do not describe a change as fully validated when a required test or build failed.
- A packaging failure caused only by a running application locking an existing executable must still be reported separately from successful compilation and tests.

## Git operations

- Never create a Git commit unless the user explicitly asks for a commit in the current request.
- Never push commits, branches, or tags unless the user explicitly confirms the push in the current request.
- Do not infer permission to commit or push from earlier requests or completed code changes.
- Leave completed changes uncommitted by default.
- When handing work back without a commit, report the validation performed and state that the changes remain uncommitted.
- Do not rewrite history, delete branches, remove tags, or force-push unless the user explicitly requests that exact operation.

## Commit conventions

- When the user explicitly requests a commit, use Conventional Commits:

  ```text
  type(scope): precise summary
  ```

- Choose a type and optional scope that accurately describe the change, such as `feat`, `fix`, `refactor`, `test`, `docs`, `build`, `ci`, or `chore`.
- Use an imperative, precise summary that describes the actual outcome.
- Include a concise commit body when needed to explain important behavior changes, architectural decisions, migrations, formulas, tests, or user-visible effects.
- Avoid vague messages such as `update`, `fix stuff`, `changes`, or `cleanup`.
- Before committing, inspect the staged diff and ensure unrelated user changes are not included accidentally.

## Releases

- An explicit release request containing a version, such as `release 0.1.1`, authorizes the complete release sequence for that version:
  1. validate the repository;
  2. inspect and stage the relevant pending changes;
  3. create a Conventional Commit for the release;
  4. push the commit;
  5. create the matching annotated version tag;
  6. push the tag;
  7. monitor the CI and release workflow until completion;
  8. report the published release and artifacts.

- Treat an explicit release request as an exception to the normal commit and push restrictions. Do not request separate confirmation for each release step.
- Use the repository's established tag format, currently `vMAJOR.MINOR.PATCH`.
- Before creating a release, verify that:
  - the working tree contains only intended changes;
  - the requested tag does not already exist;
  - required tests and builds pass;
  - the branch and remote are correct.
- Do not report a release as complete until the remote tag exists and the release workflow has successfully published its artifacts.
- If any release step fails, stop before performing materially different recovery actions and report the exact completed and pending steps.
- Never force-update an existing release tag without an explicit user request.

## Documentation

- Keep the README focused on users: application purpose, safety, installation, scanning, primary workflows, screenshots, active-development status, and links to deeper documentation.
- Keep technical explanations in focused documents under `docs`.
- Ensure documentation reflects the current code, commands, paths, UI, release process, and limitations.
- Use English for all documentation and code comments.
- Do not reference obsolete repositories, discarded prototypes, private research paths, or local development artifacts.
- Clearly state when optimization is approximate and explain the practical trade-offs between search modes.
- Verify local documentation links and referenced screenshots after reorganizing files.

## Security and privacy

- Collect, store, and expose only the data required for the application's stated purpose.
- Do not add telemetry, analytics, remote uploads, or external data transmission without explicit user approval.
- Do not commit secrets, credentials, account identifiers, private captures, or personal paths.
- Treat all imported and captured content as untrusted input.
- Validate file paths, JSON data, and external process results at application boundaries.
- Keep destructive filesystem operations narrowly scoped and verify their targets before execution.
