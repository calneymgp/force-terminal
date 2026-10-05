# Force agent runtime implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `subagent-driven-development` or the repository's `dev-coding` process to implement this plan task by task. Steps use checkboxes for tracking. This file is a plan, not authorization to run a paid CLI, contact SSH hosts, or read conversations or credentials.

**Goal:** Complete priorities 3–4 of [EVOLUTION.md](../../docs/force-terminal/EVOLUTION.md): an explicit **Novo agente** action and a safe **Reconectar agente** action that retain the same project, profile snapshot, destination, logical terminal, and exact Claude conversation ID.

**Architecture:** `ForceService` owns one durable instance and a serialized operation per instance. A Claude adapter stages a private prompt file on the execution host and launches an interactive PTY with a UUID saved before spawn. Existing block, shell, job, connection, and object-store facilities supply terminal I/O and SSH job reattachment; the agent path must suppress their normal automatic start on block restoration.

**Tech stack:** Go backend/object store and RPC, existing `shellexec`/`blockcontroller`/`jobcontroller`, React/TypeScript frontend, Go tests with a controlled fake `claude` executable.

**Spec:** `docs/force-terminal/EVOLUTION.md` priorities 3–4 and `docs/force-terminal/CLI-ADAPTER-CONTRACT.md`. Start from catalog commit `f0c935a0a38508fa091ba7dfb6cbe0abfd84939e` (projects and profiles). The overall 0–9 product objective remains open after this plan.

## Global constraints

- Saving a project/profile or creating an inert instance never launches a CLI. Only an explicit **Novo agente**, **Reconectar agente**, or separate **Nova sessão** action can do so.
- Claude Code interactive PTY is the first adapter candidate because the verified CLI documents `--session-id`, `--append-system-prompt-file`, and exact `--resume`. Codex stays a catalog choice with an explicit **execution unavailable** capability until its own thread-ID contract and UI are implemented.
- No `--last`, inferred scrollback identity, automatic fresh-session fallback, changed CLI home/history, or silent profile update on resume.
- Preserve user data and concurrent work. Do not log prompts, credentials, environment values, or session contents. Do not call a paid CLI or SSH host during automated tests.
- One active writer lease per canonical destination/worktree. Profiles do not grant filesystem permissions. Multiple writers in one project need distinct explicit checkouts; permit read-only coexistence only when the adapter enforces that mode. If canonicalization cannot establish that two paths are distinct, reject the second writer or require a different checkout; never use a disconnect as proof of process death.
- Each task uses one RED → GREEN behavior slice, a narrow test, and an atomic review/commit at execution time. Do not mark these boxes complete from documentation alone.

## File and interface map

| Area | Files to create or extend | Responsibility |
|---|---|---|
| Durable model | `pkg/waveobj/forcecatalog.go` or adjacent `forceagent.go`; `pkg/waveobj/wtype.go`; `pkg/service/forceservice/forceservice.go` | Instance, execution attempts, immutable profile snapshot, ID/capability status, service commands, object registration. Reuse `wstore.WithTx` and version checks; check generic object storage before adding a SQL migration. |
| Claude command boundary | `pkg/service/forceservice/claudeadapter.go` and `claudeadapter_test.go` | Build validated argv, stage prompt on the execution host, launch/resume through existing PTY facilities, expose process evidence. One adapter; no generic plugin framework. |
| Terminal and SSH | `pkg/blockcontroller/{blockcontroller,shellcontroller,durableshellcontroller}.go`, `pkg/jobcontroller/jobcontroller.go`, `pkg/shellexec/shellexec.go`, remote file RPC only where necessary | Keep block inert until explicit action, attach a confirmed live SSH job, link each new process attempt to the same block, and surface actual status. Reuse `StartRemoteShellJob`, `ReconnectJob`, `Job.AttachedBlockId`, terminal block file, and `RemoteWriteFileCommand` after checking file mode and atomicity. |
| UI/API | `frontend/app/force/force-catalog-model.ts`, `force-sidebar.tsx`, `frontend/app/store/services.ts`, generated `frontend/types/gotypes.d.ts` and service typings | Instance list and explicit buttons with disabled/in-progress states; existing terminal block/layout remains the visible surface. Regenerate bindings through the repository generator, not by hand. |
| Tests | `pkg/service/forceservice/*_test.go`, focused controller/job tests, frontend test only if existing harness supports interaction | Fake CLI trace, persistence, idempotency, recovery, conflict, and no fallback. |

The proposed public surface is `CreateAgentInstance(projectID, profileID, tabID, creationKey)`, `StartAgent(instanceID, requestKey)`, `ReconnectAgent(instanceID, requestKey)`, `StartNewSession(instanceID, requestKey)`, and `ListAgentInstances(projectID)`. `CreateAgentInstance` persists an inert record/block. `StartAgent` and `ReconnectAgent` return `{instance, operation, terminalBlockID}` and may report `pending` while status arrives through the existing update/event path. `requestKey` is stable for one user click; backend uniqueness and a per-instance mutex protect all windows. Exact signatures may match repository service conventions, but these semantics must survive.

`ForceAgentInstance` needs at least: stable instance/project/profile/block/tab IDs; profile version plus immutable prompt text/hash and adapter version; configured connection, user/CLI-history context and root; last **confirmed** cwd with its source (or root-only); requested Claude UUID; identity evidence (`requested` versus `observed/validated`); current operation generation, request key and phase; current/prior job or process attempt IDs; writer-lease key; timestamps and safe error code. Persist these before launch. Keep credentials as references to existing facilities, never in the instance. A new process is a new execution attempt under the same instance and block; a new conversation is a separate explicit action with a new UUID.

## Task 1 — Durable inert instance and writer reservation

**Read first:** `pkg/waveobj/forcecatalog.go`, `pkg/waveobj/wtype.go`, `pkg/service/forceservice/forceservice.go`, `pkg/wstore/wstore.go`, `pkg/wcore/block.go`.

**Interfaces produced:** `CreateAgentInstance`, `ListAgentInstances`, and an internal transaction `reserveOperation(instanceID, requestKey, intent) → (generation, priorAttempt, error)`; `intent` is `start`, `reconnect`, or `new-session`. The operation generation increases monotonically and is persisted with the UUID/snapshot/destination before any process call.

- [ ] **RED:** Add a service test that creates a project/profile, creates an instance twice with the same creation key, restarts the store, and asserts one instance, same logical block, same frozen prompt/version, a UUID, and **zero** fake process launches. Also test stale version and archived/missing project/profile rejection.
- [ ] **GREEN:** Register the instance type in `wtype.go`; persist it and its immutable snapshot with `wstore.WithTx`. Create/reuse a terminal block in an inert state. `CreateAgentInstance` may place it in the project layout but cannot set metadata that lets `ResyncController` start a process.
- [ ] **RED:** Test two simultaneous instances targeting the same host/user/canonical root. First writer reservation succeeds; second reports a conflict before launch. A disconnected SSH job keeps its lease; confirmed termination releases it. Distinct checkouts succeed.
- [ ] **GREEN:** Reserve/release a durable writer lease transactionally with the operation. Normalize paths on the execution host where possible; aliases or uncertain identity fail closed. Reconcile stale reservations only from confirmed process/job absence. Do not claim full filesystem isolation: broader worktree automation belongs to priority 7.
- [ ] **Proof:** Run the focused `go test ./pkg/service/forceservice ./pkg/waveobj` and demonstrate DB reopen, idempotent creation, zero launches on save/restore, monotonic generation, and a conflicting writer result.

## Task 2 — Controlled Claude argv and host-local prompt file

**Read first:** `docs/force-terminal/CLI-ADAPTER-CONTRACT.md`, `pkg/shellexec/shellexec.go`, `pkg/wshrpc/wshremote/wshremote_file.go`, `pkg/wshrpc/wshserver/wshserver.go`.

**Interfaces produced:** an internal `ClaudeLaunchSpec{sessionID, promptSnapshot, connection, userContext, cwd, mode}` and `prepareClaudeLaunch(spec) → (argv, promptPath, cleanup, error)`; `mode=start|resume`. The execution adapter must return process/job evidence, not infer success from terminal text.

- [ ] **RED:** Fake CLI records its argv, cwd, synthetic host/user marker, prompt-file bytes/hash, and attempt ID to a test-owned trace. Assert new session uses exact preallocated UUID and prompt file; resume uses `--resume <same UUID>` and the compatible snapshot. Assert there is no `--last`, shell interpolation of a path with spaces, or paid executable invocation.
- [ ] **GREEN:** Build argv from fixed flags and data, using existing shell quoting/PTY helpers where shell wrapping is unavoidable. Keep the prompt file private (`0700` parent, `0600` file), write atomically on the **same host/user** that runs Claude, and verify hash before spawn. `WriteTempFileCommand` is local-only; the SSH route needs a private remote file operation with checked mode and atomic rename, not a local path passed over SSH. Restore the same CLI history context; do not override `HOME`/`CODEX_HOME` as an implicit workaround.
- [ ] **Proof:** Focused fake CLI test covers local and simulated remote staging, spaces/unicode in paths, prompt snapshot immutability after profile edit, missing CLI, bad destination, and failed staging. No real Claude/SSH call.

## Task 3 — Explicit start and stable terminal

**Read first:** `pkg/blockcontroller/blockcontroller.go:151`, `shellcontroller.go:85`, `durableshellcontroller.go:139`, `pkg/jobcontroller/jobcontroller.go:613`, `pkg/wcore/block.go:61`, `frontend/app/view/term/term-model.ts`.

**Interfaces produced:** `StartAgent` consumes Task 1 reservation and Task 2 launch spec. It reports requested UUID separately from observed/validated identity, and links one execution attempt to the stable instance/block.

- [ ] **RED:** Restore an inert agent block and assert zero fake launches, even if `ResyncController` runs. Click Start twice (including requests from two windows) and assert one generation, one process, one block, one UUID. Crash after checkpoint but before process result and assert recovery reports `unknown/pending` rather than spawning again.
- [ ] **GREEN:** Implement the backend operation in this order: validate instance/lease/destination → reserve generation and UUID/snapshot → stage prompt → launch controlled PTY → persist process/job reference and source-backed state → attach terminal updates to existing block. A successful asynchronous `ShellController.Start` call is only `launch_requested`; wait for runtime/job evidence before `running`. For local, use a `cmd` block with `cmd:runonstart=false` and only explicit forced start, or a smaller guarded agent controller. For SSH, use `StartRemoteShellJob` with the block ID and attach its Job; set `shell` controller only once a job reference exists. Guard `DurableShellController.Start` so an agent block with missing JobId cannot auto-create a job during restore. Preserve the same term file and layout.
- [ ] **Proof:** Focused fake local/remote runner tests show the same block ID after closing/reopening the UI, distinct instance/session IDs across projects, no command as a side effect of catalog save, and no duplicate on repeated clicks. Verify no controller force-restart path can kill a confirmed live agent as a side effect of a second click.

## Task 4 — Exact reconnect with process reconciliation

**Read first:** `pkg/jobcontroller/jobcontroller.go:139`, `:1052`, `pkg/blockcontroller/durableshellcontroller.go:139`, `pkg/waveobj/wtype.go:317`, `frontend/app/block/durable-session-flyover.tsx`.

**Interfaces produced:** `ReconnectAgent` returns one of `reattached_live_job`, `resume_requested`, `unavailable`, or `uncertain`; evidence includes operation generation and job/process ID where known. `StartNewSession` is a separate explicit intent and UUID.

- [ ] **RED:** Fake an SSH Job whose ID, connection, start timestamp, and `AttachedBlockId` match the instance: reconnect reuses it and invokes `ReconnectJob`, with zero Claude starts. Fake a dead job/local reboot: exactly one process receives `--resume <saved UUID>` on the saved host/user/cwd. Fake missing history, bad destination, nonzero resume exit, and unknown job status: no new-session launch or changed UUID; instance remains retryable with a cause.
- [ ] **GREEN:** Under the same per-instance lock, reconcile the instance attempt with `Block.JobId` and `Job.AttachedBlockId`. Confirm a live SSH job before reattachment. Confirm absence/termination before a new process; treat network loss and `init`/ambiguous status as uncertain. Re-stage the compatible prompt when needed, validate destination and root/cwd on that host, invoke exact resume, and persist a new execution attempt linked to the same block. Use configured root if current cwd has no confirmed shell-integration event, and show that limitation rather than inventing cwd.
- [ ] **RED/GREEN:** Prove crash windows after remote job creation but before writing instance JobId by recovering via block/job attachment; prove two windows and reconnect plus start cannot race to create duplicate jobs. Make status transitions source-backed (`running`, `exited`, `disconnected`, `resume_available`, `resume_failed`, `uncertain`); silence alone never means stalled or done.
- [ ] **Proof:** Focused fake runner/job tests cover local reboot simulation, live SSH job, remote host restart, network drop, exact ID, and failed resume. Compare instance ID, logical block ID, host/user, cwd source, profile hash, and UUID before and after. A fake can verify the requested ID and argv, but cannot claim provider-confirmed conversation identity.

## Task 5 — UI, bindings, and code-ready integration gate

**Read first:** `frontend/app/force/force-catalog-model.ts`, `force-sidebar.tsx`, `frontend/app/store/services.ts`, `frontend/app/view/term/term-model.ts`, existing generated type workflow.

**Interfaces produced:** user-visible **Novo agente**, **Reconectar agente**, and separate **Nova sessão** actions; selected agent opens its stable terminal block. Codex profiles show a clear unavailable state.

- [x] **RED:** Add a frontend interaction test where saving a profile/project and restoring the view make zero Start RPC calls; two clicks with one request key render one pending operation. Reconnect failure preserves instance/block and does not call Start/NewSession.
- [x] **GREEN:** Wire generated ForceService methods into the catalog model and sidebar; show the observed status and safe error cause, disable concurrent action while an operation is pending, and route to the persistent terminal block. Show “Conversa preparada” until trustworthy observation/validation; IDs and their evidence belong in optional details. The documented Claude flags alone do not constitute a structured identity callback. Keep project/agent actions reachable before opening a generic terminal.
- [x] **Proof — local code-ready:** Run focused Go tests, repository type generation/typecheck, targeted frontend tests, and a local fake CLI walkthrough. Verified on 2026-10-05: Go and race suites, generation/typecheck, 40 frontend tests, production Electron/backend builds, 14 release-validator tests and 9/9 Linux fake-CLI UI checks passed. The persisted agent lease was demonstrably absent from the existing updater guard; it is now tested as a blocker, including when no controller is loaded. macOS packaging/smoke and all external gates remain separate and open.

## External acceptance gates — remain unchecked until authorized and observed

- [ ] **Real Claude conversation:** With explicit authorization for any paid CLI call, start a disposable conversation, record its visible context and requested UUID without reading existing conversations/credentials, terminate, resume the exact ID, and verify prior context is accessible. If interactive Claude supplies no structured identity callback, record `requested` separately and use a documented/manual validation result; never silently promote it to `validated`.
- [ ] **Real restart and SSH:** On a controlled host/project, verify app close/reopen, actual notebook reboot, SSH job alive during notebook shutdown, remote host restart, and network loss. Check the same instance/block/project/profile/root and exact session ID, with no duplicate process. These are the EVOLUTION.md acceptance checks and cannot be satisfied by a fake alone.
- [ ] **M5 installed-app acceptance:** Exercise the packaged macOS arm64 app on the M5, including persistence and terminal/SSH restoration, separately from code-ready tests and any DMG build. Existing distribution/update priority 1b gates remain independent.

## Limits and decision record

- Claude Code support is an implementation hypothesis based on the verified CLI flags, not a claim that an untested interactive resume will preserve all provider-side context. Failures preserve the instance and offer retry; only **Nova sessão** allocates a new UUID.
- `jobcontroller.ReconnectJob` has singleflight by **job ID**, not by agent instance. Start/reconnect serialization and crash reconciliation belong in `ForceService` before any job exists.
- The existing durable shell accepts SSH but not local; the normal command controller is not durable, and generic controller restoration can start a command automatically. Task 3 must prove the inert guard. If its proposed reuse cannot satisfy that proof, record the finding here and implement a focused agent controller instead of weakening the invariant.
- Writer reservation covers the resolved destination/worktree for this delivery. It does not replace worktrees, branch review, or broader isolation planned at priority 7.
- No automated test in this plan uses real Claude, Codex, SSH, user history, or credentials. No external communication or paid action is authorized by this plan.

## Status log

- 2026-10-04: Plan written from catalog commit `f0c935a0a38508fa091ba7dfb6cbe0abfd84939e` and the verified adapter contract. All implementation and external acceptance boxes remain open.
- 2026-10-05: Local implementation reviewed and tested: durable inert instance/snapshot, canonical writer lease, exact Claude argv/private prompt, dedicated guarded PTY/controller, operation receipt ledger (migration 14), original terminal focus, source-backed process-group reconciliation and updater/shutdown guards. Related Go tests and six `-race` targets passed; generation/typecheck passed; frontend 40/40 and release validators 14/14 passed. Mixed local/SSH rows above remain unchecked until their full remote scope is implemented; this is not completion of priorities 3–4. The next remote slices are in [the SSH plan](../force-agent-ssh/PLAN.md).
- Limits found: a crash before PID/boot ID evidence remains uncertain even after reboot; temp prompt cleanup after crash is not confirmed; project filtering does not yet supply one exclusive tab/layout per project. Real conversation, SSH and M5 gates remain open. Evidence and detailed limits: [runtime report](../../docs/force-terminal/AGENT-RUNTIME-REPORT.md).
