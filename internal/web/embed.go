/*
 * Copyright 2026 Jonas Kaninda
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

// Package web embeds the built dashboard (the Vue SPA) into the Go binary so a
// single `posta` executable serves both the API and the UI — no static files to
// ship alongside it and no POSTA_WEB_DIR to configure.
//
// The `dist/` directory is a build artifact: `make build-ui` runs the Vue build
// and stages its output here, then `go build` bakes it in. Only a placeholder
// (.gitkeep) is committed, so `go build` and `go test` always compile on a clean
// checkout; in that case the embedded FS holds no index.html and the UI routes
// 404 while the API serves normally. Release and Docker builds always build the
// UI first, so shipped binaries are self-contained.
package web

import "embed"

// Assets holds the built SPA under a top-level "dist/" directory. It is served
// through Okapi's WebFS with WebConfig{Root: "dist"}. The `all:` prefix keeps
// dotted files (the .gitkeep placeholder, and any dotfiles Vite emits), which
// //go:embed would otherwise skip.
//
//go:embed all:dist
var Assets embed.FS
