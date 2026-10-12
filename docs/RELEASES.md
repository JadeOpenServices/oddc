# ODDC releases

- `main` is the release channel. Every merge to `main` is a release.
- Consumers pin a release by its commit hash. There are no tags or
  signing keys.
- `main` is never force-pushed and its history is never rewritten, so a
  pinned commit stays on `main` for good.
- Recommended: GitHub branch protection on `main` that blocks force-pushes
  and deletion and requires CI to pass.

Maintainers release with `oddc release` ([WORKFLOW.md](WORKFLOW.md#maintainers)):
it checks `staging`, opens the pull request to `main`, waits for CI and
merges only after asking.

To check a pinned revision, run `oddc validate --root . --json` in a
checkout of that commit.
