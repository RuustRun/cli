# ruust CLI

`ruust` is the command line and TUI for [**Ruust**](https://ruust.run), the flat-priced
container hosting platform by **The Night Project**. Sign in, lay an Egg from a git
repo or a demo, and watch it go from incubating to hatched, from your terminal.

Written in Go with [cobra](https://github.com/spf13/cobra) for commands and the
[charm](https://github.com/charmbracelet) stack (bubbletea, lipgloss, bubbles) for the
interactive dashboard. It talks to the same public `/api/v1` the rest of Ruust uses.

## Install and build

```sh
go build -o ruust .
```

Move the `ruust` binary onto your `PATH`. Release binaries for macOS and Linux
(amd64 and arm64) are published to GitHub Releases by the release workflow.

## Commands

Running `ruust` with no subcommand opens the interactive Egg dashboard when you are
signed in.

| Command | What it does |
| --- | --- |
| `ruust` | Open the interactive Egg dashboard (needs a session) |
| `ruust login` / `logout` / `whoami` | Manage your session |
| `ruust create` | Lay a new Egg from a repo or demo |
| `ruust ls` | List your Eggs |
| `ruust open <name>` | Open an Egg's URL |
| `ruust status` | Show account and Egg status |
| `ruust logs <name>` | Stream an Egg's logs |
| `ruust version` | Print the CLI version |

The CLI currently covers the core loop (sign in, create, list, open, tail). Managing
env vars, domains, resize, Coops, teams, databases and the like still lives in the
dashboard; parity is on the roadmap.

## Configuration

Config lives at `~/.config/ruust/config.json` (owner-only, it holds a session token).
`ruust login` writes it for you. Environment variables take precedence, for scripts
and CI:

| Variable | Overrides | Notes |
| --- | --- | --- |
| `RUUST_HOST` | `host` | API host. Also settable per-run with `--host`. |
| `RUUST_TOKEN` | `token` | Session token, sent as `Authorization: Bearer`. |

## Licence

Apache-2.0. See [LICENSE](LICENSE).

British English throughout. No em dashes.
