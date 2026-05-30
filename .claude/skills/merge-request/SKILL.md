---
name: merge-request
description: Generate a GitLab Merge Request description as a markdown file in .tmp/
model: haiku
disable-model-invocation: true
argument-hint: [target-branch]
allowed-tools: Bash, Read, Grep, Glob
---

Generate a GitLab Merge Request description file.

## Instructions

1. **Identify the current branch:**
   - Run `git branch --show-current` to get the current branch name.

2. **Determine the target branch:**
   - If `$ARGUMENTS` is provided, use it as the target branch.
   - Otherwise, default to `main`.

3. **Gather all changes since diverging from the target branch:**
   - Run `git log <target-branch>..HEAD --oneline` to list all commits on this branch.
   - Run `git diff <target-branch>...HEAD --stat` to get a summary of changed files.
   - Run `git diff <target-branch>...HEAD` to see the full diff.
   - Also check `git status` for any uncommitted changes (staged or unstaged) and include them in the analysis.

4. **Analyze the changes thoroughly:**
   - Read through all commits and the full diff.
   - Understand the purpose, scope, and impact of the changes.
   - Group related changes by theme or module.

5. **Generate the Merge Request markdown file at `.tmp/merge-request.md`** with this structure:

```markdown
# <MR Title - concise, imperative mood>

## Description

<2-4 sentences explaining what this MR does and why.>

## Changes

<Bulleted list of changes grouped by module/theme. Be specific.>

## Impact

<What areas of the system are affected? Any breaking changes?>

## Testing

<How were changes tested or how should they be tested?>

## Notes

<Any additional context, trade-offs, or follow-up work needed. Remove this section if empty.>
```

6. **Output:** Write the file to `.tmp/merge-request.md` and inform the user of the file path.
