package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/deviceinsight/kubectl-actuator/cmd"
)

func main() {
	// kubectl looks for a kubectl_complete-<plugin> executable to provide a
	// plugin's shell completion; installs ship it as a symlink to this binary
	// (see README), so completion requests arrive under that name.
	binName := filepath.Base(os.Args[0])
	if strings.HasPrefix(binName, "kubectl_complete") {
		cmd.PrintCompletion()
	} else {
		cmd.Execute()
	}
}
