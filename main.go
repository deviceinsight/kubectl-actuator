package main

import (
	"github.com/deviceinsight/kubectl-actuator/cmd"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	binName := filepath.Base(os.Args[0])
	if strings.HasPrefix(binName, "kubectl_complete") {
		cmd.PrintCompletion()
	} else {
		cmd.Execute()
	}
}
