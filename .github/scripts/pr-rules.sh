#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-or-later

# Enforces the contribution rules of docs/WORKFLOW.md on one pull request.
# It reads the pull request through the GitHub API and never runs its code.
#
#   pr-rules.sh            enforce on $PR, with the environment the workflow sets
#   pr-rules.sh models     print the device models among the paths on stdin
#   pr-rules.sh entities   print those whose entity the paths on stdin change
set -euo pipefail

# models prints each device model, vendor/model, that the paths on stdin
# change: its entity or its evidence. Components and quirks are shared.
models() {
  sed -nE \
    -e 's|^catalog/entities/model/([^/]+/[^/]+)\.json$|\1|p' \
    -e 's|^evidence/model/([^/]+/[^/]+)/.*|\1|p' |
    sort -u
}

# entities prints each device model whose own entity the paths on stdin
# change.
entities() {
  sed -nE 's|^catalog/entities/model/([^/]+/[^/]+)\.json$|\1|p' | sort -u
}

case "${1:-}" in
  models | entities)
    "$1"
    exit 0
    ;;
esac

: "${REPO:?}" "${PR:?}" "${AUTHOR:?}" "${ASSOCIATION:?}" "${BASE:?}" "${HEAD_REPO:?}" "${HEAD_REF:?}" "${HEAD_SHA:?}"
rules="https://github.com/$REPO/blob/staging/docs/WORKFLOW.md#contribution-rules"

# Promotion is the only change main takes.
if [ "$BASE" = main ] && [ "$HEAD_REPO/$HEAD_REF" = "$REPO/staging" ]; then
  exit 0
fi


member=false
case "$ASSOCIATION" in
  OWNER | MEMBER | COLLABORATOR) member=true ;;
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

# A shared change moves the closure of every model using it at once, so a
# member's pull request may carry evidence for each of them; it still
# changes at most one model's own entity. Anyone else's changes one model.
files=$(gh api --paginate "repos/$REPO/pulls/$PR/files" --jq '.[].filename')
if [ "$member" = true ]; then
  changed=$(printf '%s\n' "$files" | entities)
else
  changed=$(printf '%s\n' "$files" | models)
fi
if [ "$(printf '%s' "$changed" | grep -c .)" -gt 1 ]; then
  gh pr comment "$PR" --repo "$REPO" --body "This pull request changes more than one device model:

$(printf '%s\n' "$changed" | sed 's/^/- /')

A pull request changes at most one, with the components, quirks and evidence it needs. Split it, one device each. See $rules"
  exit 1
fi

# A model change is proven on verify/<model>, branched from staging, before
# it reaches staging; the branch merges into staging once oddc status says
# verified. A change to no model, such as a shared quirk, may also target
# the verify branch of the model it is for; anything else targets staging.
target=staging
if [ -n "$changed" ]; then
  target="verify/$changed"
elif [[ $BASE == verify/* ]]; then
  target=$BASE
fi
if [ "$BASE" = "$target" ] || { [ "$BASE" = staging ] && [ "$HEAD_REPO/$HEAD_REF" = "$REPO/$target" ]; }; then
  :
else
  # The branch is named from a path in the pull request: only an ID's form.
  if ! [[ $target =~ ^(staging|verify/[a-z0-9]+(-[a-z0-9]+)*/[a-z0-9]+(-[a-z0-9]+)*)$ ]]; then
    echo "not a model ID: $changed" >&2
    exit 1
  fi
  if [ "$target" != staging ] && ! gh api "repos/$REPO/branches/$target" --silent 2>/dev/null; then
    gh api "repos/$REPO/git/refs" --silent -f "ref=refs/heads/$target" \
      -f "sha=$(gh api "repos/$REPO/branches/staging" --jq .commit.sha)"
  fi
  gh pr edit "$PR" --repo "$REPO" --base "$target"
  gh pr comment "$PR" --repo "$REPO" --body "This pull request now targets \`$target\`. A device model is proven on its own \`verify/<model>\` branch, which merges into \`staging\` once the model is verified; anything else targets \`staging\`. See $rules"
fi

# Only maintainers and collaborators verify: anyone else's evidence may
# not claim a passing status. Each file is read as data, never run.
if [ "$member" = false ]; then
  passing=$(gh api --paginate "repos/$REPO/pulls/$PR/files" \
    --jq '.[] | select(.status != "removed") | .filename' |
    { grep -E '^evidence/.*\.json$' || true; } |
    while read -r file; do
      status=$(gh api -H "Accept: application/vnd.github.raw" \
        "repos/$REPO/contents/$file?ref=$HEAD_SHA" --jq '.status' 2>/dev/null || true)
      case "$status" in
        detected | documented) ;;
        *) printf '%s (%s)\n' "$file" "${status:-unreadable}" ;;
      esac
    done)
  if [ -n "$passing" ]; then
    gh pr comment "$PR" --repo "$REPO" --body "Only maintainers and collaborators verify models, so evidence from this pull request may only be \`documented\` or \`detected\`:

$(printf '%s\n' "$passing" | sed 's/^/- /')

Record it with \`--status detected\`; a maintainer verifies the model. See $rules"
    exit 1
  fi
fi
