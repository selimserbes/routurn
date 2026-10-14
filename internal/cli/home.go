package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// runHomeMenu gives interactive users one entry point without removing the
// explicit commands that scripts and experienced users rely on.
func runHomeMenu(cmd *cobra.Command) error {
	if !pickerIsInteractiveTerminal(cmd) {
		return cmd.Help()
	}
	menu := interactiveMenu{
		Heading:     "What would you like to do?",
		ConfirmHint: "Open selected workflow",
		Items: []interactiveMenuItem{
			{Label: "Run a task", Detail: "sync, execute and collect results"},
			{Label: "Update project", Detail: "find an archive and review changes"},
			{Label: "View results", Detail: "browse previous results"},
			{Label: "Undo last update", Detail: "restore the most recent backup"},
		},
	}
	selection, err := chooseInteractiveMenu(cmd, "Home", menu)
	if err != nil {
		return err
	}
	if selection.Index < 0 {
		return nil // Cancel / back from the home screen.
	}
	var action *cobra.Command
	switch selection.Index {
	case 0:
		action = newExecCmd()
	case 1:
		action = newUpdateCmd()
	case 2:
		action = newResultCmd()
	case 3:
		action = newRollbackCmd()
	default:
		return fmt.Errorf("invalid home menu selection")
	}
	action.SetIn(cmd.InOrStdin())
	action.SetOut(cmd.OutOrStdout())
	action.SetErr(cmd.ErrOrStderr())
	return action.Execute()
}
