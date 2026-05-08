# CLI Reference

## Global Flags

- `--output text|json|yaml`: format for data-producing commands.
- `--verbose`: include additional diagnostics when available.
- `--debug`: enable debug diagnostics.
- `--no-color`: disable colored output.

## Commands

```sh
terminos init <bundle>
terminos import <bundle> --from transcript.txt
terminos capture <bundle>
terminos play <bundle> --speed 1 --step-mode enter|auto|none
terminos render <bundle> --format markdown|asciinema|script
terminos validate <bundle>              # demo.yaml and metadata.json
terminos validate <bundle> --captures
terminos list-steps <bundle>
```

## Import Transcript Format

The import command splits transcript text on prompt-prefixed command lines:

```text
$ command one
output from command one

$ command two
output from command two
```

The prompt defaults to `"$ "` and can be changed in `demo.yaml` under `defaults.prompt`.

## Playback Controls

- `--step-mode enter`: type the command, then wait for Enter before showing its captured output.
- `--step-mode auto`: type the command, then replay output using capture event timing.
- `--step-mode none`: type the command and show output immediately.
- `--type-delay 18ms`: override the per-character command typing delay.
- `--max-delay 2s`: cap output event delays in `auto` mode.
- `--speed 2`: make delays twice as fast. Values below `1` slow playback down.
- `--no-delay`: skip all playback delays and Enter waits for deterministic automation.

## Exit Codes

- `0`: success
- `1`: internal error
- `2`: usage, config, or schema parse error
- `3`: validation failed
- `4`: captured command failed
- `5`: timeout
- `6`: filesystem error
- `7`: redaction policy failure
- `130`: interrupted

## AI_CONTEXT

Command help includes an `AI_CONTEXT` section. It is plain help text for humans and a stable contract for agents and CI. It tells callers when to use JSON, which stream contains parseable data, and which exit codes have domain-specific meaning.
