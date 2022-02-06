package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/function61/gokit/jsonfile"
)

const (
	versionJsonFilename = "version.json"
)

type EnvVarSpec struct {
	Key         string `json:"key"`
	Optional    bool   `json:"optional"`
	Placeholder string `json:"placeholder"`
	Help        string `json:"help"`
}

type Version struct {
	FriendlyVersion string `json:"friendly_version"` // 20190107_1257_ec16791b
}

type DeplSpecManifest struct {
	ManifestVersionMajor        int          `json:"manifest_version_major"`     // SemVer major version
	DeployerImage               string       `json:"deployer_image"`             // fn61/infrastructureascode:20190107_1257_ec16791b
	DeployCommand               []string     `json:"deploy_command"`             // ["./deploy.sh"]
	DeployInteractiveCommand    []string     `json:"deploy_interactive_command"` // defaults to ["/bin/bash"]
	DownloadArtefacts           []string     `json:"download_artefacts"`
	DownloadArtefactUrlTemplate string       `json:"download_artefact_urltemplate"`
	EnvVars                     []EnvVarSpec `json:"env_vars"`           // user configurable stuff
	SoftwareUniqueId            string       `json:"software_unique_id"` // random UUID that should stay the same forever, used to prevent accidentally deploying wrong software
}

type UserConfig struct {
	ServiceID        string            `json:"service_id"`
	Envs             map[string]string `json:"envs"`
	SoftwareUniqueId string            `json:"software_unique_id"`
}

// below datatypes are not serialized

type VersionAndManifest struct {
	Version  Version
	Manifest DeplSpecManifest
}

type Deployment struct {
	Vam        VersionAndManifest
	UserConfig UserConfig

	// computed
	ExpandedDeployCommand            []string
	ExpandedDeployInteractiveCommand []string
}

// error is true for os.IsNotExist() if file not found
func loadUserConfig(serviceId string) (*UserConfig, error) {
	config := &UserConfig{}
	return config, jsonfile.Read(userConfigPath(serviceId), config, true)
}

func validateUserConfig(user *UserConfig, vam *VersionAndManifest) (*Deployment, error) {
	knownKeys := map[string]bool{}

	for _, env := range vam.Manifest.EnvVars {
		_, defined := user.Envs[env.Key]

		if !env.Optional && !defined {
			return nil, fmt.Errorf("ENV %s required but not defined in user config", env.Key)
		}

		knownKeys[env.Key] = true
	}

	for key := range user.Envs {
		if _, exists := knownKeys[key]; !exists {
			return nil, fmt.Errorf("unknown ENV %s defined in user config", key)
		}
	}

	if vam.Manifest.SoftwareUniqueId == "" {
		return nil, errors.New("SoftwareUniqueId cannot be empty")
	}

	if user.SoftwareUniqueId != vam.Manifest.SoftwareUniqueId {
		return nil, fmt.Errorf(
			"software ID mismatch; deploymentConfig(%s) != deploymentPackage(%s)",
			user.SoftwareUniqueId,
			vam.Manifest.SoftwareUniqueId)
	}

	expandedDeployCommand := []string{}
	for _, part := range vam.Manifest.DeployCommand {
		partExpanded, err := expandPossibleVariables(part, vam, user)
		if err != nil {
			return nil, err
		}

		expandedDeployCommand = append(expandedDeployCommand, partExpanded)
	}

	expandedDeployInteractiveCommand := []string{}
	for _, part := range vam.Manifest.DeployInteractiveCommand {
		partExpanded, err := expandPossibleVariables(part, vam, user)
		if err != nil {
			return nil, err
		}

		expandedDeployInteractiveCommand = append(expandedDeployInteractiveCommand, partExpanded)
	}

	return &Deployment{
		Vam:        *vam,
		UserConfig: *user,

		ExpandedDeployCommand:            expandedDeployCommand,
		ExpandedDeployInteractiveCommand: expandedDeployInteractiveCommand,
	}, nil
}

var variableExpansionRe = regexp.MustCompile(`\$\{([^}]+)\}`)

// "--version=${_.version.friendly}" => "--version=v314"
func expandPossibleVariables(input string, versionAndManifest *VersionAndManifest, user *UserConfig) (string, error) {
	expansion := variableExpansionRe.FindStringSubmatch(input)
	if expansion == nil {
		return input, nil
	}

	key := expansion[1]

	replace, err := func() (string, error) {
		switch {
		case key == "_.version.friendly":
			return versionAndManifest.Version.FriendlyVersion, nil
		case strings.HasPrefix(key, "_.env."):
			realKey := key[len("_.env."):]

			val, found := user.Envs[realKey]
			if !found {
				return "", fmt.Errorf("no variable defined in UserConfig: %s", realKey)
			}

			return val, nil
		default:
			return "", fmt.Errorf("unknown expansion key: %s", expansion[1])
		}
	}()
	if err != nil {
		return "", err
	}

	return strings.ReplaceAll(input, expansion[0], replace), nil
}
