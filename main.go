package main

import (
	"github.com/eckertalex/keylightctl/cmd"
)

var Version = "v0.0.0"

func main() {
	cmd.Execute(Version)
}
