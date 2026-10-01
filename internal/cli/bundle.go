package cli

import (
	"encoding/json"
	"fmt"

	"github.com/selimserbes/routurn/internal/bundle"
	"github.com/spf13/cobra"
)

type bundleInspectOptions struct {
	StripComponents int
	JSON            bool
}

func newBundleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bundle",
		Short: "Inspect update bundles without modifying a project",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newBundleInspectCmd())
	return cmd
}

func newBundleInspectCmd() *cobra.Command {
	var opts bundleInspectOptions
	cmd := &cobra.Command{
		Use:   "inspect <archive>",
		Short: "Validate and inspect an update archive",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inspection, err := bundle.InspectArchive(args[0], opts.StripComponents)
			if err != nil {
				return err
			}
			if opts.JSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(inspection)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Archive    %s\n", inspection.Archive)
			fmt.Fprintf(cmd.OutOrStdout(), "Format     %s\n", inspection.Format)
			fmt.Fprintf(cmd.OutOrStdout(), "Backend    %s\n", inspection.Backend)
			fmt.Fprintf(cmd.OutOrStdout(), "Files      %d\n", inspection.Files)
			fmt.Fprintf(cmd.OutOrStdout(), "Bytes      %d\n", inspection.Bytes)
			fmt.Fprintf(cmd.OutOrStdout(), "Executable %d\n", inspection.Executable)
			for _, entry := range inspection.Entries {
				fmt.Fprintf(cmd.OutOrStdout(), "  %04o  %10d  %s\n", entry.Mode&0o777, entry.Size, entry.Path)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "✓ Archive passed Routurn safety validation")
			return nil
		},
	}
	cmd.Flags().IntVar(&opts.StripComponents, "strip-components", 0, "remove leading path components from archive entries")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "print machine-readable JSON")
	return cmd
}
