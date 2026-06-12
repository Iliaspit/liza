package main

import (
	"fmt"
	"os"

	"github.com/liza-mas/liza/internal/commands"
	"github.com/spf13/cobra"
)

var capsuleCmd = &cobra.Command{
	Use:   "capsule",
	Short: "Manage local isolated Liza capsules",
	Long: `Manage local isolated Liza capsules.

A capsule mounts the current repository as /workspace while shadowing
/workspace/.liza with capsule-owned state. Container capsules always use a
Linux guest toolchain, even when launched from macOS or Windows hosts.`,
}

var capsuleCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a local Liza capsule",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectRoot, err := requireProjectRoot()
		if err != nil {
			return err
		}
		preset, _ := cmd.Flags().GetString("preset")
		runtimeName, _ := cmd.Flags().GetString("runtime")
		image, _ := cmd.Flags().GetString("image")
		storeRoot, _ := cmd.Flags().GetString("store-root")
		modelsDevProvider, _ := cmd.Flags().GetString("models-dev-provider")
		apiKeyEnv, _ := cmd.Flags().GetString("api-key-env")
		preferredModels, _ := cmd.Flags().GetStringArray("preferred-model")
		daytonaAPIURL, _ := cmd.Flags().GetString("daytona-api-url")
		daytonaTarget, _ := cmd.Flags().GetString("daytona-target")
		daytonaSnapshot, _ := cmd.Flags().GetString("daytona-snapshot")
		daytonaCPU, _ := cmd.Flags().GetInt("daytona-cpu")
		daytonaMemory, _ := cmd.Flags().GetInt("daytona-memory")
		daytonaDisk, _ := cmd.Flags().GetInt("daytona-disk")
		daytonaAutoStop, _ := cmd.Flags().GetInt("daytona-auto-stop")
		daytonaAutoDelete, _ := cmd.Flags().GetInt("daytona-auto-delete")
		noProvision, _ := cmd.Flags().GetBool("no-provision")
		_, err = commands.CapsuleCreateCommand(commands.CapsuleCreateParams{
			ProjectRoot:       projectRoot,
			Name:              args[0],
			Preset:            preset,
			Runtime:           runtimeName,
			Image:             image,
			StoreRoot:         storeRoot,
			Context:           cmd.Context(),
			ModelsDevProvider: modelsDevProvider,
			APIKeyEnv:         apiKeyEnv,
			PreferredModels:   preferredModels,
			DaytonaAPIURL:     daytonaAPIURL,
			DaytonaTarget:     daytonaTarget,
			DaytonaSnapshot:   daytonaSnapshot,
			DaytonaCPU:        daytonaCPU,
			DaytonaMemoryGB:   daytonaMemory,
			DaytonaDiskGB:     daytonaDisk,
			DaytonaAutoStop:   daytonaAutoStop,
			DaytonaAutoDelete: daytonaAutoDelete,
			NoProvision:       noProvision,
		})
		return err
	},
}

var capsuleDoctorCmd = &cobra.Command{
	Use:   "doctor <name>",
	Short: "Check capsule runtime, toolchain, and auth bridge readiness",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectRoot, err := requireProjectRoot()
		if err != nil {
			return err
		}
		tool, _ := cmd.Flags().GetString("tool")
		storeRoot, _ := cmd.Flags().GetString("store-root")
		_, err = commands.CapsuleDoctorCommand(commands.CapsuleDoctorParams{
			ProjectRoot: projectRoot,
			Name:        args[0],
			Tool:        tool,
			StoreRoot:   storeRoot,
		})
		return err
	},
}

var capsuleStartCmd = &cobra.Command{
	Use:   "start <name> [-- command...]",
	Short: "Start a capsule",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectRoot, err := requireProjectRoot()
		if err != nil {
			return err
		}
		storeRoot, _ := cmd.Flags().GetString("store-root")
		return commands.CapsuleStartCommand(commands.CapsuleStartParams{
			ProjectRoot: projectRoot,
			Name:        args[0],
			Command:     args[1:],
			StoreRoot:   storeRoot,
			Stdout:      os.Stdout,
			Stderr:      os.Stderr,
			Stdin:       os.Stdin,
		})
	},
}

var capsuleReportCmd = &cobra.Command{
	Use:   "report <name>",
	Short: "Create a redacted capsule report zip",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectRoot, err := requireProjectRoot()
		if err != nil {
			return err
		}
		storeRoot, _ := cmd.Flags().GetString("store-root")
		_, err = commands.CapsuleReportCommand(cmd.Context(), commands.CapsuleReportParams{
			ProjectRoot: projectRoot,
			Name:        args[0],
			StoreRoot:   storeRoot,
		})
		return err
	},
}

var capsuleStopCmd = &cobra.Command{
	Use:   "stop <name>",
	Short: "Stop a cloud capsule",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectRoot, err := requireProjectRoot()
		if err != nil {
			return err
		}
		storeRoot, _ := cmd.Flags().GetString("store-root")
		force, _ := cmd.Flags().GetBool("force")
		return commands.CapsuleStopCommand(commands.CapsuleStopParams{
			ProjectRoot: projectRoot,
			Name:        args[0],
			StoreRoot:   storeRoot,
			Force:       force,
			Context:     cmd.Context(),
		})
	},
}

var capsuleSnapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Manage cloud capsule snapshots",
}

var capsuleSnapshotCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a Daytona snapshot from an immutable image",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		imageName, _ := cmd.Flags().GetString("image")
		apiURL, _ := cmd.Flags().GetString("daytona-api-url")
		regionID, _ := cmd.Flags().GetString("region")
		sandboxClass, _ := cmd.Flags().GetString("sandbox-class")
		entrypoint, _ := cmd.Flags().GetStringArray("entrypoint")
		cpu, _ := cmd.Flags().GetInt("cpu")
		memory, _ := cmd.Flags().GetInt("memory")
		disk, _ := cmd.Flags().GetInt("disk")
		_, err := commands.CapsuleSnapshotCreateCommand(commands.CapsuleSnapshotCreateParams{
			Name:         args[0],
			ImageName:    imageName,
			APIURL:       apiURL,
			RegionID:     regionID,
			SandboxClass: sandboxClass,
			Entrypoint:   entrypoint,
			CPU:          cpu,
			MemoryGB:     memory,
			DiskGB:       disk,
			Context:      cmd.Context(),
		})
		return err
	},
}

var capsuleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List capsules for the current repository",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectRoot, err := requireProjectRoot()
		if err != nil {
			return err
		}
		storeRoot, _ := cmd.Flags().GetString("store-root")
		items, err := commands.CapsuleListCommand(projectRoot, storeRoot)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			fmt.Fprintln(os.Stdout, "No capsules found")
		}
		return nil
	},
}

var capsuleDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a local capsule",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectRoot, err := requireProjectRoot()
		if err != nil {
			return err
		}
		storeRoot, _ := cmd.Flags().GetString("store-root")
		localOnly, _ := cmd.Flags().GetBool("local-only")
		return commands.CapsuleDeleteCommandWithRemote(cmd.Context(), projectRoot, storeRoot, args[0], localOnly)
	},
}

func addCapsuleStoreFlag(cmd *cobra.Command) {
	cmd.Flags().String("store-root", "", "capsule store root (default: XDG_DATA_HOME/liza/capsules or ~/.local/share/liza/capsules)")
}

func init() {
	rootCmd.AddCommand(capsuleCmd)
	capsuleCmd.AddCommand(capsuleCreateCmd)
	capsuleCmd.AddCommand(capsuleDoctorCmd)
	capsuleCmd.AddCommand(capsuleStartCmd)
	capsuleCmd.AddCommand(capsuleReportCmd)
	capsuleCmd.AddCommand(capsuleStopCmd)
	capsuleCmd.AddCommand(capsuleSnapshotCmd)
	capsuleCmd.AddCommand(capsuleListCmd)
	capsuleCmd.AddCommand(capsuleDeleteCmd)
	capsuleSnapshotCmd.AddCommand(capsuleSnapshotCreateCmd)

	capsuleCreateCmd.Flags().String("preset", "openai-compatible", "OpenCode/provider preset")
	capsuleCreateCmd.Flags().String("runtime", "docker", "capsule runtime (docker, podman, or daytona)")
	capsuleCreateCmd.Flags().String("image", "", "capsule image name (default: liza-capsule:latest)")
	capsuleCreateCmd.Flags().String("models-dev-provider", "", "derive OpenCode provider/models from models.dev provider ID")
	capsuleCreateCmd.Flags().String("api-key-env", "", "API key environment variable for models.dev-derived provider config")
	capsuleCreateCmd.Flags().StringArray("preferred-model", nil, "preferred model ID for models.dev-derived provider config (repeatable)")
	capsuleCreateCmd.Flags().String("daytona-api-url", "", "Daytona API URL (default: https://app.daytona.io/api)")
	capsuleCreateCmd.Flags().String("daytona-target", "", "Daytona target/region, such as us or eu")
	capsuleCreateCmd.Flags().String("daytona-snapshot", "", "Daytona snapshot/image for cloud capsules")
	capsuleCreateCmd.Flags().Int("daytona-cpu", 0, "Daytona sandbox CPU cores")
	capsuleCreateCmd.Flags().Int("daytona-memory", 0, "Daytona sandbox memory in GB")
	capsuleCreateCmd.Flags().Int("daytona-disk", 0, "Daytona sandbox disk in GB")
	capsuleCreateCmd.Flags().Int("daytona-auto-stop", 30, "Daytona auto-stop interval in minutes")
	capsuleCreateCmd.Flags().Int("daytona-auto-delete", -1, "Daytona auto-delete interval in minutes (-1 disables)")
	capsuleCreateCmd.Flags().Bool("no-provision", false, "write Daytona capsule metadata without creating the remote sandbox")
	capsuleDoctorCmd.Flags().String("tool", "all", "tool to check (opencode, codex, claude, all)")
	capsuleStopCmd.Flags().Bool("force", false, "force stop the Daytona sandbox")
	capsuleSnapshotCreateCmd.Flags().String("image", "", "immutable image reference to snapshot, such as ghcr.io/org/liza-capsule:20260610")
	capsuleSnapshotCreateCmd.Flags().String("daytona-api-url", "", "Daytona API URL (default: https://app.daytona.io/api)")
	capsuleSnapshotCreateCmd.Flags().String("region", "", "Daytona region ID where the snapshot should be available")
	capsuleSnapshotCreateCmd.Flags().String("sandbox-class", "linux-vm", "Daytona sandbox class for the snapshot")
	capsuleSnapshotCreateCmd.Flags().StringArray("entrypoint", []string{"sleep", "infinity"}, "snapshot entrypoint argument (repeatable)")
	capsuleSnapshotCreateCmd.Flags().Int("cpu", 0, "default snapshot CPU cores")
	capsuleSnapshotCreateCmd.Flags().Int("memory", 0, "default snapshot memory in GB")
	capsuleSnapshotCreateCmd.Flags().Int("disk", 0, "default snapshot disk in GB")
	capsuleDeleteCmd.Flags().Bool("local-only", false, "delete local capsule metadata without deleting the remote Daytona sandbox")

	for _, cmd := range []*cobra.Command{capsuleCreateCmd, capsuleDoctorCmd, capsuleStartCmd, capsuleReportCmd, capsuleStopCmd, capsuleListCmd, capsuleDeleteCmd} {
		addCapsuleStoreFlag(cmd)
	}
}
