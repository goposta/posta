/*
 *  MIT License
 *
 * Copyright (c) 2026 Jonas Kaninda
 *
 *  Permission is hereby granted, free of charge, to any person obtaining a copy
 *  of this software and associated documentation files (the "Software"), to deal
 *  in the Software without restriction, including without limitation the rights
 *  to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 *  copies of the Software, and to permit persons to whom the Software is
 *  furnished to do so, subject to the following conditions:
 *
 *  The above copyright notice and this permission notice shall be included in all
 *  copies or substantial portions of the Software.
 *
 *  THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 *  IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 *  FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 *  AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 *  LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 *  OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 *  SOFTWARE.
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
