# Branching & Release Strategy

## Branch Hierarchy
```
development  ──▶  main  ──▶  release
   │              │          │
   │              │          └── Tagged releases only (vX.Y.Z)
   │              └── Merge from development (fast-forward only)
   └── Integration branch (CI runs on every push)
```

## Rules
1. **development**: All work lands here. CI runs on every push/PR.
2. **main**: Protected. Only fast-forward merges from `development`. No direct commits.
3. **release**: Protected. Only fast-forward merges from `main`. **Every release promotion = tag** (`vX.Y.Z`).

## Release Flow
1. Merge feature branches → `development`
2. CI passes on `development`
3. `git checkout main && git merge --ff-only development`
4. `git checkout release && git merge --ff-only main`
5. `git tag vX.Y.Z release && git push origin vX.Y.Z`

## Hotfix Path
1. `git checkout release -b hotfix/X`
2. Fix, test, commit
3. Open PR against `release` (or `git checkout release && git merge --ff-only hotfix/X`)
4. `git tag vX.Y.Z release`
5. Backport to `main` → `development` via cherry-pick/merge

## Rollback
1. `git tag` on `release` → find previous tag (e.g., `v1.2.3`)
2. `git checkout release && git reset --hard v1.2.3 && git push origin release --force-with-lease`
3. Redeploy from tag

## No Direct Commits
- Never commit directly to `main` or `release`
- All changes flow: `feature → development → main → release`
