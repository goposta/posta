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
	cli.DefaultCommand("server")

	if err := cli.Execute(); err != nil {
		logger.Fatal(err.Error())
	}
}
