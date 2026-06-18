## 2026-06-15 - Task: Merge CyberStrikeAI-main differences into CyberStrikeAI-pr
### What was done
- Merged the current file contents from `D:\OneDrive\006\project\CyberStrikeAI-main` into `CyberStrikeAI-pr`, excluding Git metadata and non-business directory noise.
- Added a Windows-only terminal websocket fallback so the merged tree builds on Windows while keeping Unix PTY websocket behavior unchanged.
- Documented the terminal API platform behavior under `docs/terminal.md`.
- Created a rollback point before merging: branch `codex/backup-before-main-merge-20260615-002109` and patch backups under `D:\OneDrive\006\project\_codex_backups\CyberStrikeAI-pr-main-merge-20260615-002109`.
### Testing
- `go test ./...` passed.
- `git diff --check` and `git diff --cached --check` passed; Git only reported CRLF conversion warnings.
- Verified normalized file comparison between `CyberStrikeAI-main` and `CyberStrikeAI-pr`: `same=560 diff=0 only_main=0 only_pr=1`; the only extra file in `CyberStrikeAI-pr` is `internal/handler/terminal_ws_windows.go`.
### Notes
- `README.md` / `README_CN.md`: merged documentation differences from `CyberStrikeAI-main`.
- `run.sh`: merged background-run script differences from `CyberStrikeAI-main`.
- `PR_DESCRIPTION.md`: added PR description from `CyberStrikeAI-main`.
- `agents/*.md`: merged the 16 agent instruction differences from `CyberStrikeAI-main`.
- `skills/tool-usage/SKILL.md`: merged the tool-usage skill content from `CyberStrikeAI-main`.
- `internal/database/conversation.go`, `internal/database/group.go`, `internal/database/project_stats.go`: merged conversation ordering related differences from `CyberStrikeAI-main`.
- `web/static/css/style.css`, `web/static/i18n/en-US.json`, `web/static/i18n/zh-CN.json`, `web/static/js/chat-scroll.js`, `web/static/js/chat.js`, `web/static/js/projects.js`, `web/static/js/webshell.js`, `web/templates/index.html`: merged frontend differences from `CyberStrikeAI-main`.
- `internal/handler/terminal_ws_windows.go`: added a Windows build fallback for the terminal websocket route.
- `docs/terminal.md`: documented terminal API endpoints and the Windows 501 behavior for interactive websocket terminals.
- Rollback: restore the pre-merge commit with `git switch fix/task-restart-state-repair` then `git reset --hard codex/backup-before-main-merge-20260615-002109`; if staged work must be restored exactly, apply the saved patches from `D:\OneDrive\006\project\_codex_backups\CyberStrikeAI-pr-main-merge-20260615-002109`.

## 2026-06-15 - Task: Clear CRLF warnings for staged merge files
### What was done
- Added repository-level line-ending rules for the current merge scope so staged text files remain LF even when global Git `core.autocrlf=true`.
- Re-staged the current merge changes after applying the line-ending rules.
- Restored unrelated knowledge-base Markdown files that were briefly touched by an overly broad renormalization attempt, keeping them out of this task.
### Testing
- `git diff --cached --check` passed.
- Verified staged worktree files contain no CRLF line endings: `NO_CRLF_IN_STAGED_WORKTREE_FILES`.
- `go test ./...` passed.
### Notes
- `.gitattributes`: added scoped LF rules for the staged merge files and `.gitattributes` itself.
- `progress.md`: appended this CRLF cleanup record.
- Rollback: remove `.gitattributes` and this `progress.md` entry, then run `git add -A`; the broader pre-merge rollback point remains `codex/backup-before-main-merge-20260615-002109`.
