package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type heapdumpCommandOperations struct {
	baseOperations
	outputFile string
	live       bool
	liveSet    bool
}

func NewHeapDumpCommand(configFlags *genericclioptions.ConfigFlags, podResolver PodResolver) *cobra.Command {
	operations := &heapdumpCommandOperations{
		baseOperations: baseOperations{
			k8sCliFlags: configFlags,
			podResolver: podResolver,
		},
	}

	cmd := &cobra.Command{
		Use:     "heapdump",
		Aliases: []string{"heap-dump"},
		Short:   "Download a JVM heap dump",
		Long: `Download a heap dump from the JVM via Spring Boot Actuator.

The dump is written to heapdump-<pod>-<timestamp>.hprof unless
--output-file is given; use '-' to stream to stdout.

Taking a heap dump pauses the JVM and can take a while for large heaps.
The 'heapdump' endpoint must be exposed; since Spring Boot 3.5 access
is restricted by default and needs
management.endpoint.heapdump.access=unrestricted.`,
		Example: `  # Dump to heapdump-<pod>-<timestamp>.hprof
  kubectl actuator -p my-app-7d4b9c-xk2pq heapdump

  # Stream to stdout and compress on the fly
  kubectl actuator -p my-app-7d4b9c-xk2pq heapdump --output-file - | gzip > dump.hprof.gz`,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			operations.liveSet = cmd.Flags().Changed("live")
			if err := operations.resolvePods(cmd); err != nil {
				return err
			}
			if err := operations.requireSingleTargetForOutputFile(operations.outputFile); err != nil {
				return err
			}
			return operations.runForEachPodTo(cmd.Context(), os.Stderr, "get heapdump", operations.runForPod)
		},
	}

	cmd.Flags().StringVar(&operations.outputFile, "output-file", "", "File to write the heap dump to; '-' for stdout (default: heapdump-<pod>-<timestamp>.hprof)")
	cmd.Flags().BoolVar(&operations.live, "live", false, "Only dump live objects (forces a full GC); omit for the JVM default")

	return cmd
}

func (o *heapdumpCommandOperations) runForPod(ctx context.Context, client actuator.Client, podName string) error {
	var live *bool
	if o.liveSet {
		live = &o.live
	}

	// Status lines always go to stderr: stdout may carry the dump itself,
	// and file mode stays consistent with that.
	if o.outputFile == "-" {
		_, _ = fmt.Fprintf(os.Stderr, "Requesting heap dump from pod %q...\n", podName)
		written, err := client.DownloadHeapDump(os.Stdout, live)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(os.Stderr, "Wrote %s to stdout\n", formatBytesHuman(float64(written)))
		return nil
	}

	fileName := o.outputFile
	if fileName == "" {
		fileName = fmt.Sprintf("heapdump-%s-%s.hprof", podName, time.Now().Format("20060102-150405"))
	}

	file, err := os.Create(fileName)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(os.Stderr, "Requesting heap dump from pod %q...\n", podName)
	written, err := client.DownloadHeapDump(file, live)
	closeErr := file.Close()
	if err != nil {
		_ = os.Remove(fileName)
		return err
	}
	if closeErr != nil {
		return closeErr
	}

	_, _ = fmt.Fprintf(os.Stderr, "Wrote %s to %s\n", formatBytesHuman(float64(written)), fileName)
	return nil
}
