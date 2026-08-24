@ /skein-verify (Post-implementation test execution)

@debugger — you own this command. You may fix test files, but NOT production code.

**You ARE the verifier.** Do not run `bin/skein verify` — that enqueues another session. Run tests directly.

@ Anti-hallucination

Before writing the report: `ls openspec/changes/<slug>/` — confirm it exists.
Confirm every file you name in the report exists first.
If required tooling is missing, say so: `Tooling: missing <tool>`. Emit `VERIFY_FAIL`.

**Pass/fail authority:** Only test exit codes and missing tooling force `VERIFY_FAIL`. File-read errors are informational only.

@ Steps

**Step 0 — Context**
1. Resolve `openspec/changes/<slug>/`.
2. Read `.skein/coder-context.md` if it exists.
3. Read `tasks.md` for any task-level test commands.

**Step 1 — Test command (Go-only repository)**
This repository is **Go only**: `go.mod` is at the root, and there is **no** `package.json` and **no** JavaScript/TypeScript code anywhere in the tree.
- If `.skein/test-command.txt` exists and is non-empty, use exactly that command.
- Otherwise the test command is exactly: `go test ./...`
- **Do NOT run `bun test`, `npm test`, `node`, `yarn`, or `pnpm`.** There is no JS/TS test suite in this repo. Running one fails with `0 test files matching ...` — that is a false negative, not a real test failure. Do not let the presence of an installed `bun` binary change the command.

**Step 2 — Run**
Run in order and record each exit code:
1. `go build ./...`
2. `go test ./...`
3. `go vet ./...`
Plus any task-specific validation commands listed in `tasks.md`.

**Step 3 — Write report**
Write via bash heredoc to `openspec/changes/<slug>/.skein/debugger-report.md`:
```bash
mkdir -p openspec/changes/<slug>/.skein
cat > openspec/changes/<slug>/.skein/debugger-report.md <<'REPORT'
...
REPORT
```

@ Report format

```
# Debugger report — <slug> — iteration <N>
## Summary
- Status: PASS | FAIL
- Commands run: <N>
## Commands
### <command>
- Exit: <code>
- Output: <last 40 lines>
## Missing coverage
<scenarios with no test, if any>
## Next coder focus
<PASS: "All tests pass." | FAIL: 2–5 bullets referencing actual errors>
```

@ Completion promise

Before the promise token, write `run-result.json`:
```bash
mkdir -p openspec/changes/<slug>/.skein
# PASS:
cat > openspec/changes/<slug>/.skein/run-result.json <<'JSON'
{"outcome":"VERIFY_PASS","requested_controls":["delegate:debugger","delegate:analyst"],"observed_behaviors":["delegate:debugger","delegate:analyst"],"delegated_roles":["debugger","analyst"]}
JSON
# FAIL:
cat > openspec/changes/<slug>/.skein/run-result.json <<'JSON'
{"outcome":"VERIFY_FAIL","requested_controls":["delegate:debugger","delegate:analyst"],"observed_behaviors":["delegate:debugger","delegate:analyst"],"delegated_roles":["debugger","analyst"]}
JSON
```

Emit exactly one as the last line:
- `<promise>VERIFY_PASS</promise>` — all commands exited 0.
- `<promise>VERIFY_FAIL</promise>` — any command failed or tooling missing.
