// Command stickit pins sticky notes to files, for humans and AI agents.
//
//	stickit add <file[:line[-line]]> "body"
//	stickit ls  [path] ["keyword"]
//	stickit resolve <id>
package main

import (
	"os"

	"github.com/williamfzc/stickit/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
