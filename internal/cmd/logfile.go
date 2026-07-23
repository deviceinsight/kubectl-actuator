package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type logfileCommandOperations struct {
	baseOperations
	outputFile string
	tailBytes  int64
}

func NewLogFileCommand(configFlags *genericclioptions.ConfigFlags, podResolver PodResolver) *cobra.Command {
	operations := &logfileCommandOperations{
		baseOperations: baseOperations{
			k8sCliFlags: configFlags,
			podResolver: podResolver,
		},
	}

	cmd := &cobra.Command{
		Use:     "logfile",
		Aliases: []string{"log-file"},
		Short:   "Download the application log file",
		Long: `Download the application log file via Spring Boot Actuator.

The log is written to stdout unless --output-file is given. Requires
'logging.file.name' or 'logging.file.path' to be configured in the
application.

Use --tail-bytes to fetch only the end of a large log file.`,
		Example: `  # Print the log file
  kubectl actuator -p my-app-7d4b9c-xk2pq logfile

  # Fetch only the last 64 KiB
  kubectl actuator -p my-app-7d4b9c-xk2pq logfile --tail-bytes 65536

  # Save to a file
  kubectl actuator -p my-app-7d4b9c-xk2pq logfile --output-file app.log`,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := operations.validateFlags(); err != nil {
				return err
			}
			if err := operations.resolvePods(cmd); err != nil {
				return err
			}
			if err := operations.requireSingleTargetForOutputFile(operations.outputFile); err != nil {
				return err
			}
			return operations.runForEachPodTo(cmd.Context(), os.Stderr, "get logfile", operations.runForPod)
		},
	}

	cmd.Flags().StringVar(&operations.outputFile, "output-file", "", "File to write the log to instead of stdout")
	cmd.Flags().Int64Var(&operations.tailBytes, "tail-bytes", 0, "Only fetch the last N bytes of the log file")
	markNoFileFlags(cmd, "tail-bytes")

	return cmd
}

func (o *logfileCommandOperations) validateFlags() error {
	if o.tailBytes < 0 {
		return fmt.Errorf("--tail-bytes cannot be negative, got %d", o.tailBytes)
	}
	return nil
}

func (o *logfileCommandOperations) runForPod(ctx context.Context, client actuator.Client, podName string) error {
	if o.outputFile == "" {
		_, err := client.DownloadLogFile(os.Stdout, o.tailBytes)
		return err
	}

	file, err := os.Create(o.outputFile)
	if err != nil {
		return err
	}

	written, err := client.DownloadLogFile(file, o.tailBytes)
	closeErr := file.Close()
	if err != nil {
		_ = os.Remove(o.outputFile)
		return err
	}
	if closeErr != nil {
		return closeErr
	}

	_, _ = fmt.Fprintf(os.Stderr, "Wrote %s to %s\n", formatBytesHuman(float64(written)), o.outputFile)
	return nil
}
