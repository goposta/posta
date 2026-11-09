---
sidebar_position: 1
title: Contributing
description: How to set up a development environment and contribute to Posta
---

# Contributing

Contributions to Posta are welcome: bug reports, fixes, documentation, translations and features.
This page explains how to get a development environment running and what a pull request needs.

## Before You Start

### Issue-first policy

Open an [issue](https://github.com/goposta/posta/issues) before starting anything beyond a small
fix, so the change can be discussed before you spend time on it. Pull requests for features that
were not discussed first may be declined. Typo fixes and small, obvious bug fixes can go straight to
a pull request.

### Community forum

Questions, ideas and show-and-tell belong on the
[community forum](https://goposta.discourse.group/). It is also the right place to check whether
a feature is already planned.

### Security reporting

Do **not** report security vulnerabilities in public issues or on the forum. Email
[me@jkaninda.dev](mailto:me@jkaninda.dev) instead, as described in
[SECURITY.md](https://github.com/goposta/posta/blob/main/SECURITY.md).

## Development Setup

### Requirements

- [Go](https://go.dev/), the version named in `go.mod`
- [Node.js](https://nodejs.org/) 24 and npm
- [Docker](https://www.docker.com/), for PostgreSQL and Redis
- [golangci-lint](https://golangci-lint.run/)
- `make`

### Environment configuration

Clone the repository and create your local configuration:

```sh
git clone https://github.com/goposta/posta.git
cd posta
cp .env.example .env
```

For local work, set these in `.env`:

```sh
POSTA_ENV=dev
POSTA_DEV_MODE=true   # store emails without sending them
```

`POSTA_ADMIN_EMAIL` and `POSTA_ADMIN_PASSWORD` set the first admin account you sign in with. The
[configuration reference](../getting-started/configuration.md) lists every other setting.

### Running dependencies

Start PostgreSQL and Redis in Docker:

```sh
make dev-deps        # postgres:17 on :5432, redis:7 on :6379
make dev-deps-stop   # stop and remove them when you are done
```

### Running API, worker and UI

Run each in its own terminal:

```sh
make dev          # API server on http://localhost:9000
make dev-worker   # background worker
make dev-ui       # dashboard with hot reload on http://localhost:3000
```

The dashboard dev server proxies API calls to `:9000`. Dev mode starts no worker inside the server,
so run `make dev-worker` whenever you test queued work such as sending, campaigns or webhooks.

To build a single binary with the dashboard embedded, run `make build-all`.

## Project Structure

| Path | Contents |
|---|---|
| `cmd/posta/` | Entry points: `server`, `worker`, `doctor` |
| `internal/routes/`, `internal/handlers/` | HTTP routes (with their OpenAPI docs) and handlers |
| `internal/services/` | Business logic, one package per domain |
| `internal/storage/` | PostgreSQL, Redis, repositories and migrations |
| `internal/worker/`, `internal/cron/` | Queued tasks and scheduled jobs |
| `internal/enterprise/` | Enterprise Edition code |
| `web/` | Dashboard: Vue 3, TypeScript, Vite, Pinia |
| `docs/` | This documentation site (Docusaurus) |

Requests flow from routes through middlewares and handlers to services and repositories. Keep
handlers thin: logic belongs in services, queries in repositories.

### Community Edition

Everything outside `internal/enterprise/` is the Community Edition. It must build and pass its tests
without any Enterprise code:

```sh
make build-ce
make test-ce
```

### Enterprise Edition

Enterprise code lives under `internal/enterprise/` and needs the `enterprise` build tag, which
`make build` and `make test` use by default. Core code talks to it through the `enterprise.EE`
interface; without the tag, `internal/enterprise/ce_stub.go` supplies a Community implementation.
Enterprise routes check the license and answer `402 Payment Required` when no valid license is
installed.

### Documentation

User documentation lives in `docs/docs/`. Preview it locally:

```sh
npm --prefix docs install
npm --prefix docs start
```

Add a new page to `docs/sidebars.ts` so it shows up in the navigation.

## Pull Requests

### Branching strategy

Use a short, descriptive branch name
such as `fix/webhook-retry` or `feat/form-export`.

### PR requirements

- One change per pull request, linked to the issue it addresses.
- A description of what changed and why, and how you tested it.
- Screenshots for dashboard changes.
- The [contribution license statement](#contributor-license-agreement) at the start of the
  description.
- CI passing.

### Tests and documentation

Add or update tests for the behaviour you change. Run the full set of checks before pushing:

```sh
make build && make build-ce      # both editions compile
make test && make test-ce        # tests, both editions
make vet && make lint            # vet both builds, run golangci-lint
npm --prefix web run build       # dashboard type-check and build
```

Tests that need a database read `TEST_DATABASE_DSN`. After adding or changing API routes, also run
`go test ./internal/routes/...`, which validates the OpenAPI document and the workspace permission
rules.

Update `docs/` in the same pull request when you change user-facing behaviour.

### Commit conventions

Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add live chat
fix: give forms full share, widget and endpoint URLs
docs: document deploying on Miabi
chore(deps): bump docs and web dependencies
```

Use `feat`, `fix`, `docs`, `refactor`, `test` or `chore`, with an optional scope such as
`feat(forms):`. Write the summary in the imperative mood, in lower case.

### Code formatting

- Go: format with `make fmt` (`gofmt`) and keep `make lint` clean.
- Dashboard: keep `npm --prefix web run type-check` clean.
- Comments explain *why*, never *what*: a single line stating a constraint or gotcha the code cannot
  express. Comments that restate the code are removed in review.
- Match the surrounding code's naming, error handling and patterns.
- Schema changes go through the GORM models plus a migration in `internal/storage/migration`.

### License headers

Every new Go file outside `internal/enterprise/` starts with this header:

```go
// Copyright (c) 2026 Jonas Kaninda.
//
// This file is part of Posta.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
//
// SPDX-License-Identifier: AGPL-3.0-or-later
```

Files under `internal/enterprise/` carry the Posta Enterprise License header instead; copy it from
an existing file in that directory.

## Licensing and Contributions

### Open-core model

Posta is open-core. Almost all of it is free software; a small set of Enterprise features is
commercially licensed. [LICENSING.md](https://github.com/goposta/posta/blob/main/LICENSING.md)
has the full breakdown.

### Community Edition licensing

The Community Edition, meaning everything outside `internal/enterprise/`, is licensed under the
[GNU Affero General Public License v3.0 or later](https://github.com/goposta/posta/blob/main/LICENSE)
(`AGPL-3.0-or-later`).

### Enterprise licensing

Files under `internal/enterprise/`, and any file carrying the Posta Enterprise License header, are
licensed under the
[Posta Enterprise License](https://github.com/goposta/posta/blob/main/internal/enterprise/LICENSE.md),
not the AGPL.

### Commercial licensing

Posta is also offered under a commercial license for anyone the AGPL does not suit. The client SDKs,
the n8n node and the Helm chart are separate projects under the Apache License 2.0. For commercial
licensing, contact the maintainers.

### Contributor License Agreement

So that contributions can be distributed under all of these licenses, every pull request must start
with the following statement:

```
By creating this pull request, I grant Jonas Kaninda and any entity authorized by him, including successors and assigns, a perpetual, worldwide, non-exclusive, irrevocable, royalty-free, transferable and sublicensable license to use, reproduce, modify, create derivative works of, distribute, and otherwise exploit these contributions under any terms, including the AGPL-3.0-or-later, commercial, and Posta Enterprise licenses.

This grant includes a license under any patent claims I own or control that are necessarily infringed by my contributions or their combination with the project.

I represent that I have the legal right to grant these rights and that my contributions are original work or appropriately licensed for submission.
```

Larger contributions may be asked to sign a more formal contributor license agreement.

## Getting Help

### Community forum

For questions about developing on Posta, ask on the
[community forum](https://goposta.discourse.group/). Bugs and feature requests go to
[GitHub issues](https://github.com/goposta/posta/issues).

### Maintainers

Posta is maintained by [Jonas Kaninda](https://github.com/jkaninda). Reach the maintainers through
the forum or an issue. For security reports, follow the
[security policy](#security-reporting); for commercial licensing, contact the maintainers directly.
