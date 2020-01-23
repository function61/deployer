package main

import (
	"github.com/function61/gokit/assert"
	"strings"
	"testing"
)

func TestValidateUserConfig(t *testing.T) {
	deployment, err := validateUserConfig(&UserConfig{
		SoftwareUniqueId: "8386d692-97bb-47ef-a682-f7139172c240",
	}, &VersionAndManifest{
		Version: Version{
			FriendlyVersion: "v314",
		},
		Manifest: DeplSpecManifest{
			DeployCommand: []string{
				"deploy_website.sh",
				"--version=${_.version.friendly}",
			},
			SoftwareUniqueId: "8386d692-97bb-47ef-a682-f7139172c240",
		},
	})
	assert.Ok(t, err)

	assert.EqualString(t, strings.Join(deployment.ExpandedDeployCommand, " "), "deploy_website.sh --version=v314")
}
