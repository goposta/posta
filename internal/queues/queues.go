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

// Package queues is a leaf so worker and cron/jobs share queue names; a drifted literal sends work nowhere.
package queues

const (
	// Transactional carries work a person is waiting on: API and relayed sends, inbound, web form messages.
	Transactional = "transactional"
	// Bulk carries campaign work, which is large and can afford to wait.
	Bulk = "bulk"
	// Low carries scheduled background work such as daily reports.
	Low = "low"
)
