---
name: CONCH_TASK
description: Manage tasks and their dependencies in the conch task tracking system. Use when creating tasks, updating task status, querying blocked or blocking tasks, managing the dependency graph, or planning work against a ticket.
---

## Overview

Tasks are the atomic units of work in conch. Each task belongs to a ticket and has a status, a body with full context, and optional dependency relationships to other tasks. The agent MUST use this skill to manage all task state — users do not interact with tasks directly.

All operations are performed via the `conch task` CLI. All output is JSON.

## Status Values

A task status MUST be one of:

- `todo` — not yet started (default)
- `in-progress` — actively being worked
- `done` — completed successfully
- `human-intervention` — blocked, requires a human to unblock

## Subcommands

### create

Create a new task under a ticket.

```
conch task create --ticket <id> --title <title> --body <context>
```

Response: `{"ok":true,"id":<task_id>}`

The agent MUST provide a `--body` with sufficient context for the task to be executed without additional clarification.

### get

Fetch a single task by ID.

```
conch task get --task <id>
```

Response: `{"ok":true,"task":{...}}`

### update-status

Update the status of a task.

```
conch task update-status --task <id> --status <status>
```

Response: `{"ok":true}`

### list

List all tasks for a ticket.

```
conch task list --ticket <id>
```

Response: `{"ok":true,"tasks":[...]}`

### add-dep

Declare that one task blocks another. The blocker MUST be completed before the blocked task can start.

```
conch task add-dep --blocker <id> --blocked <id>
```

Response: `{"ok":true}`

### remove-dep

Remove a dependency relationship.

```
conch task remove-dep --blocker <id> --blocked <id>
```

Response: `{"ok":true}`

### list-blocked-by

List all tasks that must complete before the given task can start.

```
conch task list-blocked-by --task <id>
```

Response: `{"ok":true,"tasks":[...]}`

### list-blocks

List all tasks that the given task is currently blocking.

```
conch task list-blocks --task <id>
```

Response: `{"ok":true,"tasks":[...]}`

## Task Execution Workflow

When picking up a task to work on, the agent MUST follow this sequence:

1. Run `list-blocked-by` for the task. If any blocker has status other than `done`, the agent MUST NOT start the task. It SHOULD move on to another unblocked task or halt.
2. Run `update-status --status in-progress` before beginning any work.
3. Perform the work described in the task `body`.
4. On successful completion, run `update-status --status done`.
5. If the task cannot be completed without human input, run `update-status --status human-intervention` and MUST include a clear explanation in a session log or output.

The agent MUST NOT skip step 2 — status MUST reflect actual work state at all times.

## Dependency Management

When planning work for a ticket, the agent SHOULD:

1. Run `list --ticket <id>` to get all tasks.
2. For each task, run `list-blocked-by` to understand the dependency graph before deciding execution order.
3. Use `add-dep` to record dependencies discovered during planning. Dependencies SHOULD be added before execution begins, not during.
4. Use `remove-dep` only when a dependency is determined to be incorrect or no longer relevant.

The agent MUST NOT create circular dependencies. Before calling `add-dep --blocker A --blocked B`, the agent SHOULD verify that B does not already (directly or transitively) block A.

## Error Handling

All responses include `"ok": true` on success. On failure, the response contains `"ok": false` and an `"error"` string. The agent MUST check `ok` before proceeding and SHOULD surface the error message when halting.
