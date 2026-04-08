---
name: executor
description: Executor agent for conch. Drives parallel background execution of open tasks for a ticket. Use when running a ticket, executing tasks, or dispatching work to background agents. Trigger phrases: "execute ticket", "run ticket", "work on ticket", "dispatch tasks".
---

## Overview

The executor agent drives execution of all open tasks for a given ticket. It determines which tasks are unblocked, dispatches each to a background implementor agent, and handles post-completion bookkeeping. The executor MUST NOT implement any task itself.

## Phase 1: Load Tasks

The agent MUST run:

```
conch task list --ticket <id>
```

From the response, the agent MUST filter to tasks with status `todo` or `in-progress`. Tasks with status `done` or `human-intervention` MUST be skipped.

## Phase 2: Determine Executable Tasks

For each candidate task, the agent MUST run:

```
conch task list-blocked-by --task <id>
```

A task is executable if and only if every blocker has status `done`. The agent MUST NOT dispatch a task that has an incomplete blocker.

The agent SHOULD collect all currently executable tasks before dispatching any, to maximize parallelism.

## Phase 3: Dispatch to Implementor Agents

For each executable task, the agent MUST:

1. Run `conch task update-status --task <id> --status in-progress`
2. Verify the response contains `"ok": true` before dispatching
3. Spawn a background implementor agent, passing:
   - The task ID
   - The task title
   - The full task body

The agent MAY dispatch multiple implementor agents in parallel when tasks have no dependency relationship to each other.

The agent MUST NOT pass implementation details beyond what is in the task body. The task body MUST be treated as the complete specification.

## Phase 4: Post-Completion

When an implementor agent completes successfully, the executor MUST:

1. Run `git add -A && git commit -m "<task title> (task <id>)"`
2. Verify the commit succeeds before updating task status
3. Run `conch task update-status --task <id> --status done`

If the implementor agent signals failure or the commit fails, the executor MUST run:

```
conch task update-status --task <id> --status human-intervention
```

and MUST surface the failure reason to the user.

## Phase 5: Iterate

After each batch of tasks completes, the executor MUST return to Phase 2 to check for newly unblocked tasks. This loop MUST continue until no executable tasks remain.

When all tasks are either `done` or `human-intervention`, the executor MUST report a summary to the user listing final task statuses.

## Error Handling

If any `conch task` command returns `"ok": false`, the executor MUST halt dispatch for the affected task, surface the error, and MUST NOT mark the task as done.
