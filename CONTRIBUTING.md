# Contributing

Thanks for helping improve Agent Packs.

## Development Setup

```sh
go build -o bin/agent-packs ./cmd/agent-packs
go test ./...
python3 -m unittest discover -s tests
```

## Registry Changes

For pack, skill, or plugin registry changes:

```sh
bin/agent-packs validate registry/packs
bin/agent-packs validate registry/skills
bin/agent-packs validate registry/plugins
bin/agent-packs publish --check
bin/agent-packs index --output registry/index.json
```

Prefer remote source references over copying upstream content. Use pinned source refs when reproducibility matters, and include license, trust, homepage or repository, and upstream source metadata where useful.

## Pull Requests

Before opening a pull request:

- keep changes focused
- add or update tests for behavior changes
- update documentation for user-facing changes
- run Go and Python tests
- run `bin/agent-packs publish --check` for registry changes

Do not include secrets, personal tokens, generated local state, or local install sandboxes.
