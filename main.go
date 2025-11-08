package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/deviceinsight/kubectl-actuator/cmd"
)

func main() {
	binName := filepath.Base(os.Args[0])
	if strings.HasPrefix(binName, "kubectl_complete") {
		cmd.PrintCompletion()
	} else {
		cmd.Execute()
	}
}
