// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
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
