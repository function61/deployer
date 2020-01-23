package main

import (
	"errors"
	"fmt"
	"github.com/function61/gokit/fileexists"
	"github.com/function61/gokit/jsonfile"
	"github.com/satori/go.uuid"
	"io"
)

func deploymentCreateConfig(serviceId string, url string) error {
	// safety check, because the below logic will create dirs
	deploymentDirExists, err := fileexists.Exists("deployments")
	if err != nil {
		return err
	}
	if !deploymentDirExists {
		return errors.New("deployments/ directory does not exist - aborting for safety\nIf this is the right location and you're running Deployer for the first time, run:\n\t$ mkdir deployments/")
	}

	deploymentConfigExists, err := fileexists.Exists(userConfigPath(serviceId))
	if err != nil {
		return err
	}

	if deploymentConfigExists {
		return errors.New("deployment config already exists - it'd be dangerous to overwrite")
	}

	// .. but the deployment for this service must not exist

	vam, err := downloadAndExtractSpecByUrl(serviceId, url)
	if err != nil {
		return err
	}

	userEnvs := map[string]string{}

	for _, manifestEnv := range vam.Manifest.EnvVars {
		val := manifestEnv.Placeholder
		if manifestEnv.Optional {
			val = "(optional) " + val
		}

		userEnvs[manifestEnv.Key] = val
	}

	if err := jsonfile.Write(userConfigPath(serviceId), &UserConfig{
		ServiceID:        serviceId,
		Envs:             userEnvs,
		SoftwareUniqueId: vam.Manifest.SoftwareUniqueId,
	}); err != nil {
		return err
	}

	fmt.Printf("Wrote %s\n", userConfigPath(serviceId))

	return nil
}

func manifestStubCreate(out io.Writer) error {
	manifest := &DeplSpecManifest{
		ManifestVersionMajor: 1,
		DeployerImage:        "fn61/infrastructureascode:20190107_1257_ec16791b",
		DeployCommand:        []string{"/work/deploy.sh"},
		DownloadArtefacts:    []string{},
		EnvVars: []EnvVarSpec{
			{
				Key:         "MY_AWESOME_API_KEY",
				Placeholder: "Looks like AKIAI..",
				Help:        "Your API key for AWS",
			},
			{
				Key:      "OPTIONAL_KEY",
				Optional: true,
				Help:     "Set to 'foo' to charge the flux capacitor",
			},
		},
		SoftwareUniqueId: uuid.NewV4().String(),
	}

	return jsonfile.Marshal(out, manifest)
}
