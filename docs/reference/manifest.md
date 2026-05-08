# Manifest Reference

`demo.yaml` declares the demo steps and playback defaults. `metadata.json` records the terminos version and host details from the last bundle write. `terminos validate` checks both files; `terminos validate --captures` also checks capture readiness for play and render.

```yaml
schema_version: "1.0"
name: hello
summary: A starter terminos demo.
defaults:
  shell: bash
  cwd: .
  prompt: "$ "
  timeout: 30s
  terminal:
    cols: 120
    rows: 32
playback:
  step_mode: enter
  type_delay: 18ms
  max_delay: 2s
redactions:
  - pattern: "/home/[^ ]+"
    replacement: "~"
steps:
  - id: hello
    title: Print a friendly message
    command: printf 'hello from terminos\n'
    expect_exit: 0
```

Step IDs must match `^[a-z][a-z0-9_-]*$`. If a step omits `capture`, terminos uses `captures/<step-id>.jsonl`.

`playback.step_mode` controls the default `terminos play` behavior for the bundle:

- `enter`: type the command, then wait for Enter before showing captured output.
- `auto`: type the command, then replay output using capture event timing.
- `none`: type the command and show output immediately.

The CLI flag `--step-mode enter|auto|none` overrides the manifest default for one run.

Capture files are JSONL streams. The MVP writes `start`, `stdout`, `stderr`, and `exit` events:

```json
{"type":"start","time_ms":0,"step_id":"hello","command":"printf 'hello\\n'","cwd":"examples/hello","shell":"bash"}
{"type":"stdout","time_ms":1,"step_id":"hello","data":"hello\n"}
{"type":"exit","time_ms":1,"step_id":"hello","exit_code":0}
```

Bundle metadata is stable JSON:

```json
{
  "schema_version": "1.0",
  "tool_version": "0.1.0-dev",
  "updated_at": "2026-05-08T00:00:00Z",
  "host": {
    "os": "linux",
    "arch": "amd64",
    "go": "go1.25.0"
  }
}
```
