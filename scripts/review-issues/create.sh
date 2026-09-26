#!/usr/bin/env bash
#
# File the code-review findings as GitHub issues.
#
#   ./scripts/review-issues/create.sh --dry-run    # show what would be created
#   ./scripts/review-issues/create.sh              # create everything
#
# Idempotent: milestones and labels are skipped if they already exist, and any
# issue whose title is already present in the repo is skipped too, so you can
# re-run after a partial failure without creating duplicates.

set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANIFEST="$HERE/manifest.json"
BODIES="$HERE/bodies"
CREATED="$HERE/created.txt"

DRY_RUN=0
[[ "${1:-}" == "--dry-run" ]] && DRY_RUN=1

command -v gh >/dev/null 2>&1 || { echo "gh not found. Install: https://cli.github.com" >&2; exit 1; }
[[ -f "$MANIFEST" ]] || { echo "No manifest. Run: python3 $HERE/generate.py" >&2; exit 1; }
gh auth status >/dev/null 2>&1 || { echo "Not logged in. Run: gh auth login --web" >&2; exit 1; }

REPO="$(python3 -c "import json;print(json.load(open('$MANIFEST'))['repo'])")"
echo "Target repo: $REPO"
echo

created_count=0
skipped_count=0
: > "$CREATED"

# ---------------------------------------------------------------------------
# Milestones
# ---------------------------------------------------------------------------
existing_milestones="$(gh api "repos/$REPO/milestones?state=all&per_page=100" \
  --paginate --jq '.[].title' 2>/dev/null || true)"

echo "── Milestones ──────────────────────────────────────────"
while IFS=$'\t' read -r title description; do
  [[ -z "$title" ]] && continue
  if grep -Fxq "$title" <<<"$existing_milestones"; then
    printf '  = %-14s (exists)\n' "$title"
  elif (( DRY_RUN )); then
    printf '  + %-14s %s\n' "$title" "$description"
  else
    gh api "repos/$REPO/milestones" -f title="$title" -f description="$description" >/dev/null
    printf '  + %-14s %s\n' "$title" "$description"
  fi
done < <(python3 -c "
import json
for m in json.load(open('$MANIFEST'))['milestones']:
    print(m['title'] + '\t' + m['description'])
")
echo

# ---------------------------------------------------------------------------
# Labels
# ---------------------------------------------------------------------------
existing_labels="$(gh api "repos/$REPO/labels?per_page=100" --paginate --jq '.[].name' 2>/dev/null || true)"

echo "── Labels ──────────────────────────────────────────────"
while IFS=$'\t' read -r name color description; do
  [[ -z "$name" ]] && continue
  if grep -Fxq "$name" <<<"$existing_labels"; then
    printf '  = %-14s (exists)\n' "$name"
  elif (( DRY_RUN )); then
    printf '  + %-14s #%s\n' "$name" "$color"
  else
    gh api "repos/$REPO/labels" -f name="$name" -f color="$color" -f description="$description" >/dev/null
    printf '  + %-14s #%s\n' "$name" "$color"
  fi
done < <(python3 -c "
import json
for l in json.load(open('$MANIFEST'))['extraLabels']:
    print(l['name'] + '\t' + l['color'] + '\t' + l['description'])
")
echo

# ---------------------------------------------------------------------------
# Issues
# ---------------------------------------------------------------------------
existing_titles="$(gh api "repos/$REPO/issues?state=all&per_page=100" \
  --paginate --jq '.[] | select(.pull_request == null) | .title' 2>/dev/null || true)"

echo "── Issues ──────────────────────────────────────────────"
current_tier=""
while IFS=$'\t' read -r id title milestone labels_csv; do
  [[ -z "$id" ]] && continue

  if [[ "$milestone" != "$current_tier" ]]; then
    printf '\n  [%s]\n' "$milestone"
    current_tier="$milestone"
  fi

  if grep -Fxq "$title" <<<"$existing_titles"; then
    printf '    = %s\n' "$id"
    skipped_count=$((skipped_count + 1))
    continue
  fi

  labels_args=()
  IFS=',' read -ra labs <<<"$labels_csv"
  for l in "${labs[@]}"; do labels_args+=(--label "$l"); done

  if (( DRY_RUN )); then
    printf '    + %-7s %s\n' "$id" "$title"
  else
    url="$(gh issue create --repo "$REPO" \
            --title "$title" \
            --body-file "$BODIES/$(python3 -c "import json;print([i['file'] for i in json.load(open('$MANIFEST'))['issues'] if i['id']=='$id'][0])")" \
            --milestone "$milestone" \
            "${labels_args[@]}" 2>/dev/null)"
    printf '    + %-7s %s\n' "$id" "${url:-$title}"
    echo "$id	$title	${url:-}" >> "$CREATED"
  fi
  created_count=$((created_count + 1))
done < <(python3 -c "
import json
m = json.load(open('$MANIFEST'))
for i in m['issues']:
    print(i['id'] + '\t' + i['title'] + '\t' + i['milestone'] + '\t' + ','.join(i['labels']))
")
echo

if (( DRY_RUN )); then
  echo "DRY RUN — nothing created. Re-run without --dry-run to file for real."
else
  echo "Done: $created_count issue(s) created, $skipped_count skipped."
  [[ -s "$CREATED" ]] && echo "Log: ${CREATED#$HERE/../}"
  echo
  echo "View: https://github.com/$REPO/issues?q=is%3Aissue+is%3Aopen"
fi
