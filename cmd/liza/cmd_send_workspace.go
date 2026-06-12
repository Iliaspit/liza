package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var sendWorkspaceInstallHint = strings.TrimSpace(`
Install it with:
  go install github.com/liza-mas/liza-report/cmd/liza-send-workspace@latest
and ensure the binary is on your PATH.
`)

var sendWorkspaceCmd = &cobra.Command{
	Use:                "send-workspace",
	Short:              "Send the current Liza workspace",
	Args:               cobra.ArbitraryArgs,
	DisableFlagParsing: true,
	RunE:               runSendWorkspace,
}

func init() {
	rootCmd.AddCommand(sendWorkspaceCmd)
}

func runSendWorkspace(cmd *cobra.Command, args []string) error {
	executable, err := exec.LookPath("liza-send-workspace")
	if err != nil {
		return fmt.Errorf("missing dependency: liza-send-workspace is not installed or not on PATH.\n%s", sendWorkspaceInstallHint)
	}
	child := exec.Command(executable, args...)
	child.Stdin = os.Stdin
	child.Stdout = cmd.OutOrStdout()
	child.Stderr = cmd.ErrOrStderr()
	return child.Run()
}
