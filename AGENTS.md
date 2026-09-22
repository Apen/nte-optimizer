# Repository instructions

## Git operations

- Never push commits, branches, or tags unless the user explicitly confirms the push in the current request.
- Never create a Git commit unless the user explicitly asks for a commit in the current request.
- Do not infer permission to commit or push from earlier requests or from completed code changes.
- Leave completed changes uncommitted by default and report the working-tree status to the user.
- An explicit release request that includes a version, such as "release 0.1.1", authorizes the complete release sequence for that version: commit the relevant pending changes, push the commit, create the matching version tag, and push the tag. Treat this as an explicit exception to the commit and push restrictions above; do not ask for separate confirmation for each release step.
