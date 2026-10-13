# ODDC releases

- `stable` is the release channel and the source of truth for every device:
  it is the repository's default branch, so `github:JadeOpenServices/oddc`
  with no ref follows it, and `oddc update` follows `stable` unless
  `--channel staging` or a pinned `--rev` says otherwise.
- Every merge to `stable` is a release.
- Consumers pin a release by its commit hash. There are no tags or
  signing keys.
- `stable` is never force-pushed and its history is never rewritten, so a
  pinned commit stays on `stable` for good.
- Recommended: GitHub branch protection on `stable` that blocks force-pushes
  and deletion and requires CI to pass.

Maintainers release with `oddc release` ([WORKFLOW.md](WORKFLOW.md#maintainers)):
it checks `staging`, opens the pull request to `stable`, waits for CI and
merges only after asking.

`--notes DIR` writes what goes to people: `DIR/release-notes.md` for
maintainers (the pull request and its text) and, per verified device in
the release, `DIR/<vendor>-<model>.md`, the page for the machine's
owner (what works, what works because of ODDC, what does not yet, security,
plans, how it was tested), with a QR code `DIR/<vendor>-<model>-qr.svg` and
`.png`. `--notes-url URL` is where the code leads, `{device}` standing for
`<vendor>-<model>`; without it the code leads to the pull request.

To check a pinned revision, run `oddc validate --root . --json` in a
checkout of that commit.
