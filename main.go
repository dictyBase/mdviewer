package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"mdviewer/internal/viewer"
)

func main() {
	app := &cli.App{
		Name:  "mdviewer",
		Usage: "A web server that displays markdown files as HTML",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "dir",
				Aliases: []string{"d"},
				Value:   ".",
				Usage:   "Directory containing markdown files",
			},
			&cli.IntFlag{
				Name:    "port",
				Aliases: []string{"p"},
				Value:   8888,
				Usage:   "Port to serve on",
			},
			&cli.StringFlag{
				Name:  "host",
				Value: "127.0.0.1",
				Usage: "Host interface to serve on",
			},
		},
		Action: runServer,
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runServer(cltx *cli.Context) error {
	return viewer.Run(viewer.Config{
		Directory: cltx.String("dir"),
		Port:      cltx.Int("port"),
		Host:      cltx.String("host"),
	})
}
