package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func interactive(deployment Deployment) error {
	interactiveCommand := deployment.ExpandedDeployInteractiveCommand
	if len(interactiveCommand) == 0 {
		interactiveCommand = []string{"/bin/bash"}
	}

	dockerRun, err := prepareDockerRun(deployment, interactiveCommand)
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

func deploy(deployment Deployment) error {
	dockerRun, err := prepareDockerRun(deployment, deployment.ExpandedDeployCommand)
	if err != nil {
		return err
	}

	redirectStandardStreams(dockerRun)

	if err := dockerRun.Start(); err != nil {
		return err
	}

	return dockerRun.Wait()
}

func prepareDockerRun(deployment Deployment, commandToRun []string) (*exec.Cmd, error) {
	ctx := context.TODO()

	if err := downloadArtefacts(ctx, deployment.UserConfig.ServiceID, deployment.Vam); err != nil {
		return nil, err
	}

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
	useShim := true

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
		commandToRun = append([]string{"/bin/bash", "--"}, commandToRun...)
	}

	dockerArgs = append(dockerArgs, commandToRun...)

	return exec.Command(dockerArgs[0], dockerArgs[1:]...), nil
}

func deployInternal(serviceId string, url string, asInteractive bool) error {
	userConf, err := loadUserConfig(serviceId)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(
				os.Stderr,
				"Deployment config not found for deployment %s\nPro-tip: run\n\t$ %s deployment-init %s %s\n",
				serviceId,
				os.Args[0],
				serviceId,
				url)

			return errors.New("config not found")
		} else {
			return err
		}
	}

	// we should always start with a blank slate for workdir (state dir is the only one
	// that can have state)
	if err := os.RemoveAll(workDir(serviceId)); err != nil {
		return err
	}

	vam, err := downloadAndExtractSpecByUrl(serviceId, url)
	if err != nil {
		return err
	}

	deployment, err := validateUserConfig(userConf, vam)
	if err != nil {
		return err
	}

	if asInteractive {
		if err := interactive(*deployment); err != nil {
			return err
		}
	} else {
		if err := deploy(*deployment); err != nil {
			return err
		}
	}

	return nil
}

func redirectStandardStreams(cmd *exec.Cmd) {
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
}
