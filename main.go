package main

import (
	"gitlab.device-insight.com/mwa/kubectl-actuator-plugin/cmd"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	var binName = filepath.Base(os.Args[0])
	if strings.HasPrefix(binName, "kubectl_complete") {
		cmd.PrintCompletion()
	} else {
		cmd.Execute()
	}
}
