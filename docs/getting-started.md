# Getting Started

Create a bundle:

```sh
terminos init demos/hello
```

Validate it:

```sh
terminos validate demos/hello
terminos validate demos/hello --output json
```

List steps:

```sh
terminos list-steps demos/hello
```

The initial bundle contains `demo.yaml`, `metadata.json`, and empty directories for future capture events and assets.

Capture and replay the starter step:

```sh
terminos capture demos/hello
terminos play demos/hello --step-mode none --type-delay 0s
```

Import a pasted transcript:

```sh
cat > transcript.txt <<'EOF'
$ echo hello
hello

$ printf 'bye\n'
bye
EOF

terminos import demos/imported --from transcript.txt
terminos play demos/imported --no-delay
```

Replay with timing controls:

```sh
terminos play demos/imported --step-mode auto --speed 2 --max-delay 500ms
terminos play demos/imported --step-mode enter
```

Render shareable artifacts:

```sh
terminos render demos/imported --format markdown --out imported.md
terminos render demos/imported --format asciinema --out imported.cast
terminos render demos/imported --format script --out imported.sh
```
