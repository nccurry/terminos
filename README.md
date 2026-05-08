# terminos

`terminos` turns terminal transcripts into replayable demo bundles that stay easy to review in Git.

The project is early, but the core loop works: create or import a bundle, capture command output, replay it, and render shareable artifacts.

## Quickstart

```sh
task setup
task build

./dist/terminos init demos/hello
./dist/terminos capture demos/hello
./dist/terminos play demos/hello --step-mode none --type-delay 0s
./dist/terminos validate demos/hello --output json
./dist/terminos list-steps demos/hello
./dist/terminos render demos/hello --format markdown
```

A bundle is just a directory:

```text
demos/hello/
  demo.yaml
  metadata.json
  captures/
  assets/
```

## Current Commands

- `terminos init <bundle>` creates a starter `demo.yaml`, `metadata.json`, `captures/`, and `assets/`.
- `terminos import <bundle> --from transcript.txt` converts pasted terminal output into steps and captures.
- `terminos capture <bundle>` executes manifest commands and writes capture JSONL files.
- `terminos play <bundle>` replays captured terminal steps with `--speed`, `--step-mode`, `--type-delay`, and `--max-delay` controls.
- `terminos render <bundle> --format markdown|asciinema|script` exports artifacts.
- `terminos validate <bundle>` validates `demo.yaml` and `metadata.json`.
- `terminos validate <bundle> --captures` also validates capture files.
- `terminos list-steps <bundle>` prints the manifest steps.

Data-producing commands support `--output text|json|yaml`. Command help includes an `AI_CONTEXT` section documenting stable output and exit-code behavior for automation.

## Development

```sh
task ci
task test:coverage
task cli -- validate examples/hello --output json
task release:local VERSION=0.1.0
task release:check VERSION=0.1.0
```

The Taskfile is the local and CI contract. GitHub Actions runs `task ci`.

## License

`terminos` is licensed under the GNU Affero General Public License v3.0 or later. See [LICENSE](LICENSE).
