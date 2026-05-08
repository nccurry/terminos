# Examples

These public examples are deterministic and safe to run locally.

- `hello`: one successful command.
- `multi-step`: three captured steps, including an expected non-zero exit.
- `expected-failure`: a focused expected-failure bundle.
- `imported-transcript`: a bundle created from `examples/transcript.txt`.
- `mock-cloud-rollout`: ten multiline steps for a mock service rollout.
- `mock-data-pipeline`: ten multiline steps for a mock batch pipeline.
- `mock-cli-workshop`: ten multiline steps for a mock terminal workshop.

Try them with:

```sh
terminos validate examples/multi-step --captures
terminos play examples/multi-step --step-mode none --type-delay 0s
terminos render examples/multi-step --format markdown
terminos play examples/mock-cloud-rollout --step-mode none --type-delay 0s --max-delay 0s
```
