#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-or-later

# Enforces the contribution rules of docs/WORKFLOW.md on one pull request.
# It reads the pull request through the GitHub API and never runs its code.
#
#   pr-rules.sh            enforce on $PR, with the environment the workflow sets
#   pr-rules.sh models     print the device models among the paths on stdin
set -euo pipefail

# models prints each device model, vendor/model, that the paths on stdin
# change: its entity or its evidence. Components and quirks are shared.
models() {
  sed -nE \
    -e 's|^catalog/entities/model/([^/]+/[^/]+)\.json$|\1|p' \
    -e 's|^evidence/model/([^/]+/[^/]+)/.*|\1|p' |
    sort -u
}

if [ "${1:-}" = models ]; then
  models
  exit 0
fi

: "${REPO:?}" "${PR:?}" "${AUTHOR:?}" "${ASSOCIATION:?}" "${BASE:?}" "${HEAD_REPO:?}" "${HEAD_REF:?}"
rules="https://github.com/$REPO/blob/staging/docs/WORKFLOW.md#contribution-rules"

# Promotion is the only change main takes.
if [ "$BASE" = main ] && [ "$HEAD_REPO/$HEAD_REF" = "$REPO/staging" ]; then
  exit 0
fi

if [ "$BASE" != staging ]; then
  gh pr edit "$PR" --repo "$REPO" --base staging
  gh pr comment "$PR" --repo "$REPO" --body "Every pull request targets \`staging\`, so this one now does too. See $rules"
fi

case "$ASSOCIATION" in
  OWNER | MEMBER | COLLABORATOR) ;;
  *)
    # One open pull request per contributor; the oldest one stays.
    older=$(gh pr list --repo "$REPO" --author "$AUTHOR" --state open --json number \
      --jq "[.[] | select(.number < $PR) | \"#\(.number)\"] | join(\", \")")
    if [ -n "$older" ]; then
      gh pr comment "$PR" --repo "$REPO" --body "You already have $older open. Contributors have one open pull request at a time, and a new contributor's first one is merged before another. Reopen this one once that is done. See $rules"
      gh pr close "$PR" --repo "$REPO"
      exit 0
    fi
    ;;
esac

changed=$(gh api --paginate "repos/$REPO/pulls/$PR/files" --jq '.[].filename' | models)
if [ "$(printf '%s' "$changed" | grep -c .)" -gt 1 ]; then
  gh pr comment "$PR" --repo "$REPO" --body "This pull request changes more than one device model:

$(printf '%s\n' "$changed" | sed 's/^/- /')

A pull request changes at most one, with the components, quirks and evidence it needs. Split it, one device each. See $rules"
  exit 1
fi
