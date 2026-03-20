## 0.1.2 - 2026-03-20

### Documentation
- Redesign the README homepage with badges, quick start, command map, and richer usage sections

## 0.1.1 - 2026-03-20

### Fixes
- Store local config and token cache under `~/.outlook-mail-get`
- Automatically migrate legacy project-local `./data/` state into the user data directory on command execution

## 0.1.0 - 2026-03-20

### Features
- Support direct installation via `go install github.com/wujunyi792/outlook-mail-get@latest`

### Refactor
- Restructure the CLI into standard Cobra subcommands for fetch, accounts, and auth flows
