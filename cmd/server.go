package cmd

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fulmenhq/goneat/internal/ops"
	srv "github.com/fulmenhq/goneat/internal/server"
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Manage goneat auxiliary servers",
	Long: `Server commands inspect and manage auxiliary goneat services running on
localhost, such as the guardian approval server. These commands help
identify existing listeners, verify health, and facilitate future
background daemon support.`,
}

var serverListCmd = &cobra.Command{
	Use:   "list",
	Short: "List managed goneat servers",
	RunE:  runServerList,
}

var serverStatusCmd = &cobra.Command{
	Use:   "status [name]",
	Short: "Display status of managed servers",
	Args:  cobra.RangeArgs(0, 1),
	RunE:  runServerStatus,
}

func init() {
	rootCmd.AddCommand(serverCmd)

	capabilities := ops.GetDefaultCapabilities(ops.GroupSupport, ops.CategoryEnvironment)
	if err := ops.RegisterCommandWithTaxonomy("server", ops.GroupSupport, ops.CategoryEnvironment, capabilities, serverCmd, "Inspect and manage goneat auxiliary servers"); err != nil {
		panic(fmt.Sprintf("Failed to register server command: %v", err))
	}

	serverCmd.AddCommand(serverListCmd)
	serverCmd.AddCommand(serverStatusCmd)

	// TODO: Introduce server start/stop management with optional --daemon flag in future release.
}

func runServerList(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true

	infos, err := srv.List()
	if err != nil {
		return err
	}

	if len(infos) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No managed servers found.")
		return nil
	}

	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })

	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tPORT\tPID\tVERSION\tSTARTED")
	for _, info := range infos {
		started := "-"
		if !info.StartedAt.IsZero() {
			started = info.StartedAt.Format(time.RFC3339)
		}
		fmt.Fprintf(tw, "%s\t%d\t%d\t%s\t%s\n",
			info.Name,
			info.Port,
			info.PID,
			dashIfEmpty(info.Version),
			started,
		)
	}
	_ = tw.Flush()
	return nil
}

func runServerStatus(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	var targets []srv.Info

	switch len(args) {
	case 0:
		infos, err := srv.List()
		if err != nil {
			return err
		}
		targets = infos
	case 1:
		name := strings.TrimSpace(args[0])
		if name == "" {
			return fmt.Errorf("server name cannot be empty")
		}
		info, err := srv.Load(name)
		if err != nil {
			return err
		}
		if info == nil {
			fmt.Fprintf(cmd.OutOrStdout(), "No metadata found for server %q.\n", name)
			return nil
		}
		targets = append(targets, *info)
	default:
		return fmt.Errorf("status command accepts at most one server name")
	}

	if len(targets) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No managed servers found.")
		return nil
	}

	client := &http.Client{Timeout: 2 * time.Second}

	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tPORT\tSTATE\tVERSION\tDETAILS")

	for _, info := range targets {
		state := "unknown"
		version := dashIfEmpty(info.Version)
		details := ""

		if info.Port <= 0 {
			state = "invalid"
			details = "metadata missing port"
		} else {
			hello, err := srv.ProbeHello(info, client)
			if err != nil {
				state = "unreachable"
				details = err.Error()
			} else {
				state = "running"
				if hello.Version != "" {
					version = hello.Version
				}
				if !hello.StartedAt.IsZero() {
					details = fmt.Sprintf("started %s", hello.StartedAt.Format(time.RFC3339))
				} else if !info.StartedAt.IsZero() {
					details = fmt.Sprintf("started %s", info.StartedAt.Format(time.RFC3339))
				} else {
					details = "healthy"
				}
			}
		}

		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\n",
			info.Name,
			info.Port,
			state,
			version,
			dashIfEmpty(details),
		)
	}

	_ = tw.Flush()
	return nil
}

func dashIfEmpty(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}
