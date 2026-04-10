package app

import (
	"io"

	"github.com/amxv/cricinfo-cli/internal/cli"
)

func Run(args []string, stdout, stderr io.Writer) error {
	return cli.Run(args, stdout, stderr)
}
