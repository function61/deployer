package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/alessio/shellescape"
	"github.com/function61/deployer/pkg/dstate"
)

func interactive(ctx context.Context, deployment Deployment) error {
	interactiveCommand := deployment.ExpandedDeployInteractiveCommand
	if len(interactiveCommand) == 0 {
		interactiveCommand = []string{"/bin/bash"}
	}

	dockerRun, err := prepareDockerRun(ctx, deployment, interactiveCommand)
	if err != nil {
		return err
	}

	fmt.Printf(
		"Entering interactive mode (%s)\nDeploy command would have been: %s\n",
		strings.Join(interactiveCommand, " "),
		strings.Join(deployment.ExpandedDeployCommand, " "))

	redirectStandardStreams(dockerRun)

	if err := dockerRun.Start(); err != nil {
		return err
	}

	return dockerRun.Wait()
}

func deploy(ctx context.Context, deployment Deployment) error {
	dockerRun, err := prepareDockerRun(ctx, deployment, deployment.ExpandedDeployCommand)
	if err != nil {
		return err
	}

	redirectStandardStreams(dockerRun)

	if err := dockerRun.Start(); err != nil {
		return err
	}

	return dockerRun.Wait()
}

func prepareDockerRun(
	ctx context.Context,
	deployment Deployment,
	commandToRun []string,
) (*exec.Cmd, error) {
	envsAsDocker := []string{}

	addDockerEnv := func(key string, value string) {
		envsAsDocker = append(envsAsDocker, "-e", key+"="+value)
	}

	addDockerEnv("FRIENDLY_REV_ID", deployment.Vam.Version.FriendlyVersion)

	for key, value := range deployment.UserConfig.Envs {
		addDockerEnv(key, value)
	}

	// needed if tools inside container make excessive use of symlinks, like Terraform:
	// https://twitter.com/joonas_fi/status/1129316321743855616
	useShim := true // TODO: make this opt-in?

	workDirMount := "/work"
	if useShim {
		workDirMount = shimDirectory
	}

	dockerArgs := append([]string{
		"docker",
		"run",
		"--rm",
		"-it",
		"-v", workDir(deployment.UserConfig.ServiceID) + ":" + workDirMount,
		"-v", stateDir(deployment.UserConfig.ServiceID) + ":/state",
		"--entrypoint", "", // if image specifies entrypoint, our explicit command would get confused
		"--workdir", "/work",
	}, envsAsDocker...)

	pushDockerArg := func(args ...string) { dockerArgs = append(dockerArgs, args...) }

	if useShim {
		// bind mount us (the process that is currently running) at /shim, so we can launch
		// ourselves inside the container for doing the shim dance (copy the work dir) inside container
		ourExecutable, err := os.Executable()
		if err != nil {
			return nil, err
		}

		pushDockerArg("-v", ourExecutable+":"+shimBinaryMountPoint)
	}

	pushDockerArg(deployment.Vam.Manifest.DeployerImage)

	if useShim {
		// NOTE: -- to target argv from being parsed for context of the shim
		pushDockerArg(shimBinaryMountPoint, "launch-via-shim", "--")
	}

	// len check so [0] access doesn't fail, though that shouldn't happen
	useShell := len(commandToRun) > 0 && !strings.HasPrefix(commandToRun[0], "/")

	if useShell {
		pushDockerArg("/bin/sh", "-c", strings.Join(shellEscape(commandToRun), " "))
	} else {
		dockerArgs = append(dockerArgs, commandToRun...)
	}

	//nolint:gosec // ok
	return exec.CommandContext(ctx, dockerArgs[0], dockerArgs[1:]...), nil
}

func deployInternal(
	ctx context.Context,
	serviceId string,
	releaseId string,
	asInteractive bool,
	keepCache bool,
) error {
	// we should always start with a blank slate for workdir (state dir is the only one
	// that can have state)
	if !keepCache {
		if err := os.RemoveAll(workDir(serviceId)); err != nil {
			return err
		}
	}

	userConf, err := loadUserConfig(serviceId)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(
				os.Stderr,
				"Deployment config not found for deployment %s\nPro-tip: run\n\t$ %s deployment-init %s %s\n",
				serviceId,
				os.Args[0],
				serviceId,
				releaseId)

			return errors.New("config not found")
		} else {
			return err
		}
	}

	app, err := mkApp(ctx)
	if err != nil {
		return err
	}

	if releaseId == "" { // automatically resolve latest
		var err error
		releaseId, err = resolveLatestReleaseID(userConf.Repository, app)

		if err != nil {
			return fmt.Errorf("resolve latest release: %w", err)
		}

		log.Printf("latest release ID resolved to %s", releaseId)
	}

	if err := downloadRelease(ctx, serviceId, releaseId, app); err != nil {
		return fmt.Errorf("downloadRelease: %w", err)
	}

	vam, err := loadVersionAndManifest(serviceId)
	if err != nil {
		return fmt.Errorf("loadVersionAndManifest: %w", err)
	}

	deployment, err := validateUserConfig(userConf, vam)
	if err != nil {
		return fmt.Errorf("validateUserConfig: %w", err)
	}

	if asInteractive {
		if err := interactive(ctx, *deployment); err != nil {
			return err
		}
	} else {
		if err := deploy(ctx, *deployment); err != nil {
			return fmt.Errorf("deploy: %w", err)
		}
	}

	return nil
}

func resolveLatestReleaseID(repository string, app *dstate.App) (string, error) {
	if repository == "" {
		return "", errors.New("cannot resolve latest release ID when repository unset")
	}

	for _, release := range app.State.AllNewestFirst() {
		if release.Repository == repository {
			return release.Id, nil
		}
	}

	return "", fmt.Errorf("no release found for repo %s", repository)
}

func redirectStandardStreams(cmd *exec.Cmd) {
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
}

func shellEscape(args []string) []string {
	escaped := []string{}
	for _, arg := range args {
		// TODO: audit the dependency's implementation
		escaped = append(escaped, shellescape.Quote(arg))
	}

	return escaped
}
