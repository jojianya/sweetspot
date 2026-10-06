# Branching & Release Strategy

## Branch Hierarchy
```
development  ──▶  main  ──▶  prod
   │              │          │
   │              │          └── Tagged releases only (vX.Y.Z)
   │              └── Merge from development (fast-forward only)
   └── Integration branch (CI runs on every push)
```

## Rules
1. **development**: All work lands here. CI runs on every push/PR.
2. **main**: Protected. Only fast-forward merges from `development`. No direct commits.
3. **prod**: Protected. Only fast-forward merges from `main`. **Every prod promotion = tag** (`vX.Y.Z`).

## Release Flow
1. Merge feature branches → `development`
2. CI passes on `development`
3. `git checkout main && git merge --ff-only development`
4. `git checkout prod && git merge --ff-only main`
5. `git tag vX.Y.Z prod && git push origin vX.Y.Z`

## Hotfix Path
1. `git checkout prod -b hotfix/X`
2. Fix, test, commit
3. Open PR against `prod` (or `git checkout prod && git merge --ff-only hotfix/X`)
4. `git tag vX.Y.Z prod`
5. Backport to `main` → `development` via cherry-pick/merge

## Rollback
1. `git tag` on `prod` → find previous tag (e.g., `v1.2.3`)
2. `git checkout prod && git reset --hard v1.2.3 && git push origin prod --force-with-lease`
3. Redeploy from tag

## No Direct Commits
- Never commit directly to `main` or `prod`
- All changes flow: `feature → development → main → prod`
