# Batch Task Automation

## Purpose
Define durable queued execution for multiple prompts so operators can run sequential security work manually or on a recurring schedule.

## Requirements
### Requirement: Durable queue execution
Batch queues SHALL persist their definition and per-task outcome across operator sessions.

#### Scenario: Queue is inspected after partial completion
- **WHEN** an operator retrieves an existing queue after one or more tasks ran
- **THEN** the queue exposes persisted task status, linked conversation IDs, and recorded outcomes

### Requirement: Controlled cron recurrence
Cron-enabled queues SHALL compute a next run and reset for the next cycle without changing queue identity.

#### Scenario: Cron queue finishes a cycle
- **WHEN** a cron-enabled queue completes its current round
- **THEN** the system computes the next due time and prepares the queue for the next scheduled run

### Requirement: Queue creation validation
The system SHALL validate queue scheduling mode and task content before creating a durable queue.

#### Scenario: Invalid cron queue is submitted
- **WHEN** a queue is created with `scheduleMode=cron` and an invalid or missing cron expression
- **THEN** queue creation is rejected

#### Scenario: Queue contains empty tasks
- **WHEN** the submitted queue contains blank task entries
- **THEN** only valid task entries become durable queue tasks

### Requirement: Sequential task progression
Tasks within a queue SHALL execute one at a time in queue order.

#### Scenario: Queue runner processes tasks
- **WHEN** a queue enters running state
- **THEN** the runner executes the current task, persists its outcome, advances the current index, and only then proceeds to the next task

### Requirement: Operator queue control
Operators SHALL be able to pause or cancel a queue without losing already persisted outcomes.

#### Scenario: Queue is paused during execution
- **WHEN** an operator pauses a running queue
- **THEN** the queue enters paused state and completed task outcomes remain durable

#### Scenario: Queue is cancelled during execution
- **WHEN** an operator cancels a running queue
- **THEN** the current task is cancelled and the queue reaches a terminal cancelled state

### Requirement: Task failure isolation
Failure of one task SHALL be recorded without corrupting other queue tasks.

#### Scenario: One task fails
- **WHEN** a task execution ends in failure
- **THEN** that task is marked failed with error detail and the queue may continue according to its sequential runner semantics

## Overview
Batch task automation allows operators to submit multiple prompts as a durable queue for sequential execution under a chosen role and execution mode. Queues may be started manually or scheduled on a cron cadence, and each task retains its own conversation linkage and terminal outcome.

## Capabilities
### Capability 1: Durable Queue Definition
- The system must persist queue metadata including title, role, execution mode, scheduling mode, and task list.
- The system must persist each task as its own addressable execution unit under the queue.

### Capability 2: Sequential Queue Execution
- The system must execute tasks sequentially within a queue.
- The system must support both single-agent and multi-agent execution modes at queue level.

### Capability 3: Operator Queue Control
- The system must support queue start, pause, cancellation, schedule enable/disable, task mutation, and queue deletion.
- The system must expose queue list and queue detail views sufficient for UI progress tracking.

### Capability 4: Durable Per-Task Outcome
- The system must persist each task's linked conversation ID, current status, final result text, and error detail.
- The system must preserve per-task evidence even after the queue moves to later tasks.

### Capability 5: Cron Recurrence
- The system must compute and persist the next due time for cron-enabled queues.
- The system must reset cron queues for the next cycle without changing queue identity.

## Interfaces
- `POST /api/batch-tasks`
- `GET /api/batch-tasks`
- `GET /api/batch-tasks/:queueId`
- `POST /api/batch-tasks/:queueId/start`
- `POST /api/batch-tasks/:queueId/pause`
- `PUT /api/batch-tasks/:queueId/schedule-enabled`
- `PUT /api/batch-tasks/:queueId/tasks/:taskId`
- `POST /api/batch-tasks/:queueId/tasks`
- `DELETE /api/batch-tasks/:queueId/tasks/:taskId`
- `DELETE /api/batch-tasks/:queueId`

## State Machine
- Queue lifecycle:
  - `Pending`
  - `Running`
  - `Paused`
  - `Completed`
  - `Cancelled`
- Task lifecycle:
  - `Pending`
  - `Running`
  - `Completed` or `Failed` or `Cancelled`
- Cron overlay:
  - `Completed` with schedule enabled -> `Pending` for next cycle after reset and next-run calculation

## Data Flow
1. An operator defines a queue and its task list.
2. The queue is persisted with normalized schedule and agent-mode metadata.
3. A manual start or scheduler trigger selects the queue for execution.
4. Each task runs sequentially, creating its own conversation if needed and persisting user/assistant messages plus process details.
5. Task results or errors are written back to queue storage.
6. The queue advances `currentIndex` until the batch ends, is paused, or is cancelled.
7. Cron queues compute the next due time and reset task state for the next round.

## Constraints
- Tasks within a queue execute sequentially, not in parallel.
- A queue must not have multiple active runners at the same time.
- Cron expressions must be valid before a queue enters scheduled mode.
- Cron queues do not auto-start when already running, paused, or cancelled.
- Queue-level role and agent mode apply consistently to all tasks in that queue unless the platform introduces explicit per-task overrides in the future.
- Task evidence belongs to the linked conversation and must remain accessible even if queue metadata later changes.

## Failure Handling
- Invalid queue definitions, invalid cron expressions, or empty task lists are rejected before queue creation.
- A failed task records its failure and the queue continues to the next task unless paused or cancelled.
- Queue cancellation must cancel the currently running task and mark unfinished tasks as cancelled when appropriate.
- Scheduler errors are recorded per queue so operators can see why a scheduled run did not start.
- If a task cannot create or persist a conversation, the failure is recorded and execution advances without corrupting queue ordering.
