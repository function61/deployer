package main

import (
	"fmt"
	"github.com/function61/gokit/dynversion"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
)

func main() {
	app := &cobra.Command{
		Use:     os.Args[0],
		Short:   "Deployer deploys your projects",
		Version: dynversion.Version,
	}

	asInteractive := false
	deployCmd := &cobra.Command{
		Use:   "deploy [serviceId] [url]",
		Short: "Directly deploys the service",
		Args:  cobra.ExactArgs(2),
		Run: func(_ *cobra.Command, args []string) {
			if err := deployInternal(args[0], args[1], asInteractive); err != nil {
				panic(err)
			}
		},
	}
	deployCmd.Flags().BoolVarP(&asInteractive, "interactive", "i", asInteractive, "Enters interactive mode (prompt)")

	app.AddCommand(deployCmd)

	app.AddCommand(&cobra.Command{
		Use:   "destroy [serviceId]",
		Short: "Destroys all resources used by service",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			panic("not implemented yet")
		},
	})

	app.AddCommand(&cobra.Command{
		Use:   "package [friendlyVersion] [packageLocation]",
		Short: "Packages a spec into a zip",
		Args:  cobra.ExactArgs(2),
		Run: func(_ *cobra.Command, args []string) {
			if err := makePackage(args[0], args[1]); err != nil {
				panic(err)
			}
		},
	})

	app.AddCommand(&cobra.Command{
		Use:   "deployment-init [serviceId] [url]",
		Short: "Creates a new deployment stub for you to use",
		Args:  cobra.ExactArgs(2),
		Run: func(_ *cobra.Command, args []string) {
			if err := deploymentCreateConfig(args[0], args[1]); err != nil {
				panic(err)
			}
		},
	})

	app.AddCommand(&cobra.Command{
		Use:   "manifest-new",
		Short: "Creates a new manifest stub for you to use in new project",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, args []string) {
			if err := manifestStubCreate(os.Stdout); err != nil {
				panic(err)
			}
		},
	})

	app.AddCommand(&cobra.Command{
		Use:    "launch-via-shim [binary] [args..]",
		Hidden: true, // internal implementation
		Args:   cobra.MinimumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			if err := launchViaShim(args); err != nil {
				panic(err)
			}
		},
	})

	if err := app.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func workDir(serviceId string) string {
	return deploymentDir(serviceId) + "/work"
}

func stateDir(serviceId string) string {
	return deploymentDir(serviceId) + "/state"
}

func deploymentDir(serviceId string) string {
	abs, err := filepath.Abs("deployments/" + serviceId)
	if err != nil {
		panic(err)
	}
	return abs
}

func userConfigPath(serviceId string) string {
	return deploymentDir(serviceId) + "/user-config.json"
}
