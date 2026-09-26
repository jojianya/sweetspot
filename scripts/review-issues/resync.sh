#!/usr/bin/env bash
#
# Re-sync the bodies of already-filed review issues with the current files in
# bodies/. Use after generate.py output changes (e.g. the review moved and the
# permalinks shifted), so the live issues match what create.sh would file today.
#
#   ./scripts/review-issues/resync.sh --dry-run   # show which issues differ
#   ./scripts/review-issues/resync.sh             # update them
#
# Matches issues by exact title, using created.txt as the id -> URL map. Issues
# whose body is already identical are left alone, so this is cheap to re-run.

set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BODIES="$HERE/bodies"
MANIFEST="$HERE/manifest.json"
CREATED="$HERE/created.txt"

DRY_RUN=0
[[ "${1:-}" == "--dry-run" ]] && DRY_RUN=1

command -v gh >/dev/null 2>&1 || { echo "gh not found" >&2; exit 1; }
gh auth status >/dev/null 2>&1 || { echo "Not logged in. Run: gh auth login --web" >&2; exit 1; }
[[ -f "$MANIFEST" ]] || { echo "Run generate.py first" >&2; exit 1; }
[[ -f "$CREATED" ]] || { echo "No $CREATED — file the issues with create.sh first" >&2; exit 1; }

REPO="$(python3 -c "import json;print(json.load(open('$MANIFEST'))['repo'])")"
echo "Target repo: $REPO"

# id -> issue number
declare -A NUMBER
while IFS=$'\t' read -r id _title url; do
  [[ -z "${id:-}" ]] && continue
  n="${url##*/}"
  [[ "$n" =~ ^[0-9]+$ ]] && NUMBER["$id"]="$n"
done < "$CREATED"

echo
echo "── Resync ──────────────────────────────────────────────"
updated=0
unchanged=0
failed=0

while IFS=$'\t' read -r id title file; do
  [[ -z "${id:-}" ]] && continue
  num="${NUMBER[$id]:-}"
  if [[ -z "$num" ]]; then
    printf '    ? %-22s no issue number in created.txt — skipped\n' "$id"
    continue
  fi

  current="$(gh issue view "$num" --repo "$REPO" --json body --jq .body 2>/dev/null || true)"
  desired="$(cat "$BODIES/$file")"

  if [[ "$current" == "$(printf '%s\n' "$desired")" ]]; then
    unchanged=$((unchanged + 1))
    continue
  fi

  if (( DRY_RUN )); then
    printf '    ~ #%-4s %-22s body would change\n' "$num" "$id"
  elif gh issue edit "$num" --repo "$REPO" --body-file "$BODIES/$file" >/dev/null 2>&1; then
    printf '    + #%-4s %-22s updated\n' "$num" "$id"
  else
    printf '    ! #%-4s %-22s FAILED\n' "$num" "$id"
    failed=$((failed + 1))
  fi
  updated=$((updated + 1))
done < <(python3 -c "
import json
for i in json.load(open('$MANIFEST'))['issues']:
    print(i['id'] + '\t' + i['title'] + '\t' + i['file'])
")

echo
if (( DRY_RUN )); then
  echo "DRY RUN — $updated would change, $unchanged already current."
else
  echo "Done: $updated updated, $unchanged already current, $failed failed."
fi
