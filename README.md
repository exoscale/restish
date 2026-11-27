# Restish (Exoscale downstream)

[![CI](https://github.com/exoscale/restish/actions/workflows/ci.yaml/badge.svg)](https://github.com/exoscale/restish/actions/workflows/ci.yaml)
[![Go Reference](https://pkg.go.dev/badge/github.com/rest-sh/restish/v2.svg)](https://pkg.go.dev/github.com/rest-sh/restish/v2)
[![License](https://img.shields.io/github/license/rest-sh/restish)](LICENSE.md)

Restish is a CLI for working with REST-ish HTTP APIs. It can make generic HTTP
requests, discover OpenAPI descriptions, generate API-aware commands, manage
profiles and auth, render structured output, follow pagination links, and run
plugins.

Exoscale maintains this source-only fork of
[Restish v2](https://github.com/rest-sh/restish). It is not an official
Restish distribution. See the
[v1 upgrade guide](https://rest.sh/docs/getting-started/upgrade-from-v1/)
before replacing a v1 installation.

The downstream additions cover explicit raw output, richer security warnings,
API-backed completion and argument ordering, plugin selection and progress,
formatter configuration and interactive sessions, host formatter fallback,
hidden request-body prompts, and opt-in response middleware such as
`--rsh-anonymize` when a compatible plugin is installed.

## Try It

Make a direct request with no setup beyond installing the CLI:

```bash
restish api.rest.sh/types
```

Then format, filter, and page through real API responses:

```bash
restish api.rest.sh/images -o table --rsh-columns name,format,self
restish api.rest.sh/example -f 'body.basics.{name, label, profiles}'
```

Example table output:

```text
name                       format  self
Dragonfly macro            jpeg    /images/jpeg
Origami under blacklight   webp    /images/webp
Andy Warhol mural in Miami gif     /images/gif
```

## Install

Build this fork from source with Go 1.25.3 or newer:

```bash
git clone https://github.com/exoscale/restish.git
cd restish
go build -ldflags "-X github.com/rest-sh/restish/v2/internal/cli.Version=exoscale-$(git rev-parse --short HEAD)" -o restish ./cmd/restish
./restish --version
./restish --help
```

Homebrew, mise's `restish` shorthand, and `go install` from `rest-sh/restish`
install upstream, not this fork. The Go module path stays unchanged for plugin
and embedder compatibility.

## Why Restish?

- Use one-off HTTP commands with good defaults for headers, content types,
  retries, caching, TLS, and output.
- Connect OpenAPI descriptions to get generated API commands, help, profiles,
  auth, and shell completion.
- Filter normalized responses with shorthand or jq, then render JSON, YAML,
  tables, CSV, NDJSON, lines, gron, images, and raw downloads.
- Follow pagination links and stream API data without writing boilerplate loops.
- Extend the CLI with plugins for bulk workflows, CSV, MCP, TLS signing, and
  custom API behavior.

## Documentation

- [Documentation Home](https://rest.sh/docs/)
- [Tour of Restish](https://rest.sh/docs/getting-started/tour/)
- [Install](https://rest.sh/docs/getting-started/install/)
- [Connect to an API](https://rest.sh/docs/getting-started/connect-to-an-api/)
- [Authentication](https://rest.sh/docs/guides/authentication/)
- [Plugins](https://rest.sh/docs/plugins/)
- [Upgrade From v1](https://rest.sh/docs/getting-started/upgrade-from-v1/)

## Plugins

Bundled plugins live in this repository and can be built alongside the main
CLI:

```bash
go build ./cmd/restish-bulk
go build ./cmd/restish-csv
go build ./cmd/restish-mcp
go build ./cmd/restish-pkcs11
```

Start with [Install and Use Plugins](https://rest.sh/docs/plugins/install-and-use/)
or [Plugin Quickstart](https://rest.sh/docs/plugins/quickstart/).

For PKCS#11-backed mTLS, install `restish-pkcs11` before switching a v1 profile
to v2. Scripts that used `--rsh-pkcs11-label` or `--rsh-pkcs11-path` from the
old Exoscale PKCS#11 branch can use `--rsh-tls-signer pkcs11` with repeatable
`--rsh-tls-signer-param token_label=...` and `--rsh-tls-signer-param module=...`
options. See [TLS Signer Plugins](https://rest.sh/docs/plugins/tls-signer-plugins/).

## Development

Build from this repository when developing Restish itself:

```bash
go build ./cmd/restish
./restish --help
./restish api.rest.sh/types
```

Run the normal development loop with:

```bash
go test ./...
```

Build the CLI and bundled plugins with:

```bash
go build ./cmd/restish
go build ./cmd/restish-bulk
go build ./cmd/restish-csv
go build ./cmd/restish-mcp
go build ./cmd/restish-pkcs11
```

Run the full integration suite before releases or larger CLI/plugin changes:

```bash
go test -tags=integration ./...
```

Development docs:

- [Contributing](https://rest.sh/docs/contributing/)
- [Development Setup](https://rest.sh/docs/contributing/development-setup/)
- [Release Packaging](https://rest.sh/docs/contributing/release-packaging/)
- [Design Docs](docs/design/README.md)
- [Docs Source](site/)

## License

Restish is released under the MIT License. See [`LICENSE.md`](LICENSE.md).
