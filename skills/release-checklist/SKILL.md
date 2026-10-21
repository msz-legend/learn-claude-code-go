---
name: release-checklist
description: Prepare a small release: version, changelog, build, smoke test, and rollback. Use when cutting a tag or publishing a package.
---

# Release Checklist

Work through the list in order and record the command output as evidence.

1. **Freeze** - confirm the branch is clean and CI is green at the commit you
   intend to release.
2. **Version** - bump the version once, in the canonical file, and update any
   file that repeats it.
3. **Changelog** - list user-visible changes only. Link issues, not commits.
4. **Build** - run the documented build from a clean checkout.
5. **Smoke test** - exercise the installed artifact, not the source tree.
6. **Tag** - annotated tag with the version; push it once.
7. **Rollback** - write down the previous version and the exact command that
   restores it before announcing the release.

Do not announce until steps 4 and 5 have concrete output.
