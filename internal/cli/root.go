package cli

import (
	"fmt"

	"github.com/selimserbes/routurn/internal/remote"
	"github.com/spf13/cobra"
)

var (
	projectName string
	verbose     bool
	showVersion bool
)

func Execute(version string) error {
	root := &cobra.Command{
		Use:           "routurn",
		Short:         "Agentless remote iteration CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				fmt.Fprintln(cmd.OutOrStdout(), version)
				return nil
			}
			return cmd.Help()
		},
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			remote.SetVerbose(verbose, cmd.ErrOrStderr())
		},
	}
	root.PersistentFlags().StringVarP(&projectName, "project", "p", "", "registered project name (allows running outside the project directory)")
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "show detailed Routurn and SSH diagnostics")
	root.Flags().BoolVarP(&showVersion, "version", "V", false, "print Routurn version")

	root.AddCommand(
		newVersionCmd(version),
		newCompletionCmd(root),
		newInitCmd(),
		newDoctorCmd(),
		newTargetCmd(),
		newProjectCmd(),
		newStatusCmd(),
		newBundleCmd(),
		newApplyCmd(),
		newRollbackCmd(),
		newSyncCmd(),
		newRunCmd(),
		newLogsCmd(),
		newStopCmd(),
		newFetchCmd(),
		newExecCmd(),
		newRunsCmd(),
	)
	return root.Execute()
}

func newVersionCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print Routurn version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), version)
		},
	}
}
