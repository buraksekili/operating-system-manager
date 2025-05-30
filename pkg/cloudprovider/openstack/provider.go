/*
Copyright 2021 The Operating System Manager contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package openstack

import (
	"encoding/json"
	"errors"
	"fmt"

	providerconfig "k8c.io/machine-controller/sdk/providerconfig"
	"k8c.io/operating-system-manager/pkg/cloudprovider/openstack/types"
	"k8c.io/operating-system-manager/pkg/providerconfig/config"

	"k8s.io/klog/v2"
)

func GetCloudConfig(pconfig providerconfig.Config, kubeletVersion string) (string, error) {
	c, err := getConfig(pconfig, kubeletVersion)
	if err != nil {
		return "", fmt.Errorf("failed to parse config: %w", err)
	}

	s, err := c.ToString()
	if err != nil {
		return "", fmt.Errorf("failed to convert cloud-config to string: %w", err)
	}

	return s, nil
}
func getConfig(pconfig providerconfig.Config, kubeletVersion string) (*types.CloudConfig, error) {
	if pconfig.CloudProviderSpec.Raw == nil {
		return nil, errors.New("CloudProviderSpec in the MachineDeployment cannot be empty")
	}

	rawConfig := types.RawConfig{}
	if err := json.Unmarshal(pconfig.CloudProviderSpec.Raw, &rawConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal CloudProviderSpec: %w", err)
	}

	var (
		opts types.GlobalOpts
		err  error
	)

	// Ignore Region not found as Region might not be found and we can default it later
	opts.Region, err = config.GetConfigVarResolver().GetStringValueOrEnv(rawConfig.Region, "OS_REGION_NAME")
	if err != nil {
		klog.V(6).Infof("Region from configuration or environment variable not found")
	}

	// We ignore errors here because the OS domain is only required when using Identity API V3
	opts.DomainName, _ = config.GetConfigVarResolver().GetStringValueOrEnv(rawConfig.DomainName, "OS_DOMAIN_NAME")

	opts.AuthURL, err = config.GetConfigVarResolver().GetStringValueOrEnv(rawConfig.IdentityEndpoint, "OS_AUTH_URL")
	if err != nil {
		return nil, fmt.Errorf("failed to get the value of \"identityEndpoint\" field, error = %w", err)
	}

	trustDevicePath, _, err := config.GetConfigVarResolver().GetBoolValue(rawConfig.TrustDevicePath)
	if err != nil {
		return nil, err
	}

	// Retrieve authentication config, username/password or application credentials
	err = getConfigAuth(&opts, &rawConfig)
	if err != nil {
		return nil, err
	}

	cloudConfig := &types.CloudConfig{
		Global: opts,
		BlockStorage: types.BlockStorageOpts{
			BSVersion:       "auto",
			TrustDevicePath: trustDevicePath,
			IgnoreVolumeAZ:  true,
		},
		LoadBalancer: types.LoadBalancerOpts{
			ManageSecurityGroups: true,
		},
		Version: kubeletVersion,
	}

	if rawConfig.NodeVolumeAttachLimit != nil {
		cloudConfig.BlockStorage.NodeVolumeAttachLimit = *rawConfig.NodeVolumeAttachLimit
	}

	return cloudConfig, nil
}

func getConfigAuth(c *types.GlobalOpts, rawConfig *types.RawConfig) error {
	var err error
	c.ApplicationCredentialID, err = config.GetConfigVarResolver().GetStringValueOrEnv(rawConfig.ApplicationCredentialID, "OS_APPLICATION_CREDENTIAL_ID")
	if err != nil {
		return fmt.Errorf("failed to get the value of \"applicationCredentialID\" field, error = %w", err)
	}
	if c.ApplicationCredentialID != "" {
		klog.V(6).Infof("applicationCredentialID from configuration or environment was found.")
		c.ApplicationCredentialSecret, err = config.GetConfigVarResolver().GetStringValueOrEnv(rawConfig.ApplicationCredentialSecret, "OS_APPLICATION_CREDENTIAL_SECRET")
		if err != nil {
			return fmt.Errorf("failed to get the value of \"applicationCredentialSecret\" field, error = %w", err)
		}
		return nil
	}
	c.Username, err = config.GetConfigVarResolver().GetStringValueOrEnv(rawConfig.Username, "OS_USER_NAME")
	if err != nil {
		return fmt.Errorf("failed to get the value of \"username\" field, error = %w", err)
	}
	c.Password, err = config.GetConfigVarResolver().GetStringValueOrEnv(rawConfig.Password, "OS_PASSWORD")
	if err != nil {
		return fmt.Errorf("failed to get the value of \"password\" field, error = %w", err)
	}
	c.ProjectName, err = getProjectNameOrTenantName(rawConfig)
	if err != nil {
		return fmt.Errorf("failed to get the value of \"projectName\" field or fallback to \"tenantName\" field, error = %w", err)
	}
	c.ProjectID, err = getProjectIDOrTenantID(rawConfig)
	if err != nil {
		return fmt.Errorf("failed to get the value of \"projectID\" or fallback to\"tenantID\" field, error = %w", err)
	}
	return nil
}

// Get the Project name from config or env var. If not defined fallback to tenant name
func getProjectNameOrTenantName(rawConfig *types.RawConfig) (string, error) {
	projectName, err := config.GetConfigVarResolver().GetStringValueOrEnv(rawConfig.ProjectName, "OS_PROJECT_NAME")
	if err == nil && len(projectName) > 0 {
		return projectName, nil
	}

	// fallback to tenantName
	return config.GetConfigVarResolver().GetStringValueOrEnv(rawConfig.TenantName, "OS_TENANT_NAME")
}

// Get the Project id from config or env var. If not defined fallback to tenant id
func getProjectIDOrTenantID(rawConfig *types.RawConfig) (string, error) {
	projectID, err := config.GetConfigVarResolver().GetStringValueOrEnv(rawConfig.ProjectID, "OS_PROJECT_ID")
	if err == nil && len(projectID) > 0 {
		return projectID, nil
	}

	// fallback to tenantID
	return config.GetConfigVarResolver().GetStringValueOrEnv(rawConfig.TenantID, "OS_TENANT_ID")
}
