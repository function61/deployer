package main

import (
	"strings"
	"testing"

	"github.com/function61/gokit/assert"
)

func TestValidateUserConfig(t *testing.T) {
	deployment, err := validateUserConfig(&UserConfig{
		SoftwareUniqueId: "8386d692-97bb-47ef-a682-f7139172c240",
		Envs: map[string]string{
			"appId": "myTestApp",
		},
	}, &VersionAndManifest{
		Version: VersionFile{
			FriendlyVersion: "v314",
		},
		Manifest: DeplSpecManifest{
			DeployCommand: []string{
				"deploy_website.sh",
				"--id", "${_.env.appId}", // purposedly mixing two styles of named args
				"--version=${_.version.friendly}",
			},
			EnvVars: []EnvVarSpec{
				{
					Key: "appId",
				},
			},
			SoftwareUniqueId: "8386d692-97bb-47ef-a682-f7139172c240",
		},
	})
	assert.Ok(t, err)

	assert.EqualString(
		t,
		strings.Join(deployment.ExpandedDeployCommand, " "),
		"deploy_website.sh --id myTestApp --version=v314")
}
