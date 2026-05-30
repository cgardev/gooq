---
name: Bug report
about: Report a defect in gooq so it can be reproduced and fixed
title: "[Bug] "
labels: bug
---

## What happened

A clear and concise description of the incorrect behavior.

## Expected behavior

A clear and concise description of what you expected to happen.

## Minimal Go code sample

Provide the smallest self-contained snippet that reproduces the problem.

```go
package main

import (
	"github.com/cgardev/gooq"
)

func main() {
	// Reproduction goes here.
}
```

## Rendered SQL (if relevant)

If the issue concerns query generation, include the SQL that gooq produced and,
if helpful, the SQL you expected instead.

```sql
-- Produced:

-- Expected:
```

## Dialect

- [ ] PostgreSQL
- [ ] SQLite

## Go version

Output of `go version`:

```
```

## gooq version or commit

The released version or the commit hash you are building against:

```
```

## Additional context

Any other information, stack traces, or logs that may help diagnose the issue.
