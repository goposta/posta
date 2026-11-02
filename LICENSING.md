# Licensing

Posta is **open-core**. Most of it is free software; a small set of
Enterprise features is commercially licensed.

## Posta Community: AGPL-3.0-or-later

Posta is licensed under the **GNU Affero General Public License, version 3 or
(at your option) any later version**. See [LICENSE](./LICENSE) and
[NOTICE](./NOTICE). This covers the whole codebase except the Enterprise files
described below.

The AGPL's network clause (section 13) applies: if you run a modified Posta and
let people use it over a network, you must offer them the complete source of
your modified version under the AGPL. Running Posta unmodified, or for your own
internal use, imposes nothing beyond the usual AGPL terms.

If the AGPL does not fit your use, a commercial license is available.

## Posta Enterprise: commercial license

Enterprise features are licensed under the **Posta Enterprise License**. The
Enterprise files are everything under
[`internal/enterprise/`](./internal/enterprise), and any other file carrying
the Posta Enterprise License header. They are **not** AGPL; the terms are in
[`internal/enterprise/LICENSE.md`](./internal/enterprise/LICENSE.md).

The published images and binaries are built with the `enterprise` tag. They run
as Posta Community until a license is installed under **Admin → License**, or
with `POSTA_LICENSE` / `POSTA_LICENSE_FILE`. Licenses are verified offline:
Posta never phones home to check one.

Posta has one paid edition. A valid license unlocks every Enterprise feature of
the version you run. When it expires, Enterprise features keep running but
their settings become read-only after the grace period; sending and receiving
mail, forms and their submissions never stop because of a license.

## Which files are which

Every source file names its license with an [SPDX](https://spdx.dev) identifier:

| Files | License | SPDX identifier |
|---|---|---|
| Everything else | AGPL-3.0-or-later | `AGPL-3.0-or-later` |
| `internal/enterprise/**`, and files with the Enterprise header | Posta Enterprise License | `LicenseRef-Posta-Enterprise` |

Most Enterprise features are ordinary AGPL code that asks the license before
running (for example form routing rules and teams). Only the license
verification itself, and features that need heavy third-party dependencies,
sit behind the build tag.

## Dual licensing

Jonas Kaninda holds the copyright to Posta and offers it under the AGPL to
everyone and, separately, under a commercial license.

The client SDKs, the n8n node and the Helm chart are separate projects under the
Apache License 2.0.

For a commercial license, contact the Posta maintainers.
