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

package main

import (
	// Embedded tz database so timezone validation behaves the same on hosts without /usr/share/zoneinfo.
	_ "time/tzdata"

	"github.com/jkaninda/logger"
	"github.com/jkaninda/okapi"
	"github.com/jkaninda/okapi/okapicli"
)

func main() {
	app := okapi.New()
	cli := okapicli.New(app, "Posta")

	cli.Command("server", "Start Posta server", func(cmd *okapicli.Command) error {
		logger.Info("Starting Posta Server...")
		runServer(cli)
		return nil
	})

	cli.Command("worker", "Start Posta worker", func(cmd *okapicli.Command) error {
		logger.Info("Starting Posta Worker...")
		if err := runWorker(); err != nil {
			logger.Fatal("worker server error", "error", err)
		}
		return nil
	})
	cli.Command("doctor", "Check that this install is ready to upgrade", func(cmd *okapicli.Command) error {
		return runDoctorWorkspaces()
	})

	cli.DefaultCommand("server")

	if err := cli.Execute(); err != nil {
		logger.Fatal(err.Error())
	}
}
