package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var projectName string

func Execute(version string) error {
	root := &cobra.Command{
		Use:           "routurn",
		Short:         "Agentless remote iteration CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVarP(&projectName, "project", "p", "", "registered project name (allows running outside the project directory)")

	root.AddCommand(
		newVersionCmd(version),
		newInitCmd(),
		newDoctorCmd(),
		newTargetCmd(),
		newStatusCmd(),
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
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), version)
		},
	}
}
