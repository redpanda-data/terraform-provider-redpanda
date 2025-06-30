// Copyright 2023 Redpanda Data, Inc.
//
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

// Package utils contains multiple utility functions used across the Redpanda's
// terraform codebase
package utils

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/redpanda-data/redpanda/src/go/rpk/pkg/cloudapi"
	"github.com/redpanda-data/redpanda/src/go/rpk/pkg/plugin"
)

// ByocClientConfig represents the options that must be passed to NewByocClient.
type ByocClientConfig struct {
	AuthToken           string
	AzureSubscriptionID string
	GcpProject          string
	InternalAPIURL      string
	AzureClientID       string
	AzureClientSecret   string
	AzureTenantID       string
	GoogleCredentials   string
}

// ByocClient holds the information and clients needed to download and interact
// with the rpk byoc plugin.
type ByocClient struct {
	api                 *cloudapi.Client
	authToken           string
	internalAPIURL      string
	gcpProject          string
	azureSubscriptionID string
	azureClientID       string
	azureClientSecret   string
	azureTenantID       string
	googleCredentials   string
}

// NewByocClient creates a new ByocClient.
func NewByocClient(conf ByocClientConfig) *ByocClient {
	return &ByocClient{
		api:                 cloudapi.NewClient(conf.InternalAPIURL, conf.AuthToken),
		authToken:           conf.AuthToken,
		internalAPIURL:      conf.InternalAPIURL,
		gcpProject:          conf.GcpProject,
		azureSubscriptionID: conf.AzureSubscriptionID,
		azureClientID:       conf.AzureClientID,
		azureClientSecret:   conf.AzureClientSecret,
		azureTenantID:       conf.AzureTenantID,
		googleCredentials:   conf.GoogleCredentials,
	}
}

// RunByoc downloads and runs the rpk byoc plugin for a given cluster id and verb
// ("apply" or "destroy").
func (cl *ByocClient) RunByoc(ctx context.Context, clusterID, verb string) error {
	cluster, err := cl.api.Cluster(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("unable to request cluster details for %q: %w", clusterID, err)
	}

	byocArgs, byocEnv, err := cl.generateByocArgsAndEnv(cluster, verb)
	if err != nil {
		return err
	}

	byocPath, err := cl.getByocExecutable(ctx, cluster)
	if err != nil {
		return err
	}

	return runSubprocess(ctx, byocEnv, byocPath, byocArgs...)
}

func (*ByocClient) generateAwsArgsAndEnv() (args, env []string, err error) {
	awsArgs := []string{
		// TODO: clean this up after patch from cloud, not a good lasting solution
		"--no-validate",
	}
	return awsArgs, []string{}, nil
}

func getEnvBoolean(name string) (bool, error) {
	value := strings.ToLower(os.Getenv(name))
	if value == "" || value == "false" || value == "no" || value == "0" {
		return false, nil
	}
	if value == "true" || value == "yes" || value == "1" {
		return true, nil
	}
	return false, fmt.Errorf("bad boolean value %s=%q", name, value)
}

func (cl *ByocClient) generateAzureArgsAndEnv() (args, env []string, err error) {
	// The rpk byoc plugin does authentication a bit differently from the Terraform
	// azurerm provider. Namely, it operates in two stages: pre-flight validation
	// checks using a different Azure library, and the internal Terraform project
	// which has configurations passed into it. We want to let users use the normal
	// Terraform azurerm provider environment variables and have everything work.
	// TODO: do this in the rpk byoc plugin instead?

	azureArgs := []string{}
	azureEnv := []string{}

	// How authentication method is chosen: pre-flight validation picks an auth method
	// using --credential-source=[cli|msi|env|workload]; the internal Terraform project
	// sets use_msi, use_oidc, and use_cli provider variables based on --identity=[cli|msi|oidc];
	// and the actual Terraform azurerm provider chooses based on ARM_USE_MSI, ARM_USE_AKS_WORKLOAD_IDENTITY,
	// or ARM_USE_OIDC, or if one of ARM_CLIENT_SECRET, ARM_CLIENT_CERTIFICATE, or ARM_CLIENT_CERTIFICATE_PATH
	// are set, and otherwise uses the Azure CLI.
	authMethods := []struct {
		EnvName          string
		CredentialSource string
		Identity         string
	}{
		{"ARM_USE_MSI", "msi", "msi"},
		{"ARM_USE_OIDC", "env", "oidc"},
		{"ARM_USE_CLI", "cli", "cli"},
		// --identity=none will set all of the other use_ configs to false.
		// Terraform will correctly pick up ARM_USE_AKS_WORKLOAD_IDENTITY from the environment.
		{"ARM_USE_AKS_WORKLOAD_IDENTITY", "workload", "none"},
	}
	seenExplicitAuthMethod := false
	for _, method := range authMethods {
		use, err := getEnvBoolean(method.EnvName)
		if err != nil {
			return nil, nil, err
		}
		if use {
			if seenExplicitAuthMethod {
				return nil, nil, errors.New("only one of ARM_USE_MSI, ARM_USE_OIDC, ARM_USE_CLI, or ARM_USE_AKS_WORKLOAD_IDENTITY can be set")
			}
			seenExplicitAuthMethod = true
			azureArgs = append(azureArgs,
				"--credential-source", method.CredentialSource,
				"--identity", method.Identity,
			)
		}
	}
	if !seenExplicitAuthMethod {
		if cl.azureClientSecret != "" || os.Getenv("ARM_CLIENT_CERTIFICATE") != "" || os.Getenv("ARM_CLIENT_CERTIFICATE_PATH") != "" {
			azureArgs = append(azureArgs,
				"--credential-source", "env",
				// --identity=oidc will set use_oidc=true in the provider config which isn't what
				// we want, but is required to correctly pass client_id and client_secret through
				// to the backend config used after the first stage Terraform bootstrap. if the
				// ARM_CLIENT_ID and ARM_CLIENT_SECRET variables are defined they'll get picked
				// up automatically and it will be fine, but if AZURE_CLIENT_ID and AZURE_CLIENT_SECRET
				// are being used instead they won't get picked up and the Terraform backend will fail.
				// TODO: can change this to --identity=none after removing support for AZURE_ variables
				"--identity", "oidc",
			)
		}
	}

	// Command line arguments: pre-flight validation requires --subscription-id to be set, and
	// the internal Terraform project sets the subscription_id, client_id, and client_secret
	// provider variables based on --subscription-id, --client-id, and --client-secret.
	if cl.azureSubscriptionID == "" {
		return nil, nil, errors.New("value must be set for Azure Subscription ID")
	}
	azureArgs = append(azureArgs,
		"--subscription-id", cl.azureSubscriptionID,
		"--client-id", cl.azureClientID,
		"--client-secret", cl.azureClientSecret,
	)

	// Environment variables: pre-flight validation uses environment variables when passed
	// --credential-source=env, prefixed with AZURE_ instead of ARM_; and the internal
	// Terraform project sets the tenant_id provider variable based on AZURE_TENANT_ID.
	// Handle this by taking all the ARM_ environment variables used by the Terraform azurerm
	// provider and duplicate them as AZURE_ environment variables.
	for _, s := range os.Environ() {
		if strings.HasPrefix(s, "ARM_") {
			azureEnv = append(azureEnv, fmt.Sprintf("AZURE_%s", strings.TrimPrefix(s, "ARM_")))
		}
	}

	return azureArgs, azureEnv, nil
}

func (cl *ByocClient) generateGcpArgsAndEnv() (args, env []string, err error) {
	if cl.gcpProject == "" {
		return nil, nil, errors.New("value must be set for GCP Project")
	}
	gcpArgs := []string{
		"--project-id", cl.gcpProject,
	}

	// Handle credentials file creation.
	// Terraform's GCP provider accepts credentials directly in the GOOGLE_CREDENTIALS environment
	// variable, but rpk byoc does some pre-flight validation using a different library that
	// only allows credential paths passed in GOOGLE_APPLICATION_CREDENTIALS.
	gcpEnv := []string{}
	if cl.googleCredentials != "" {
		tempDir, err := os.MkdirTemp("", "terraform-provider-redpanda-gcp")
		if err != nil {
			return nil, nil, err
		}
		// TODO: delete temp directory after running?
		if err := os.WriteFile(path.Join(tempDir, "creds.json"), []byte(cl.googleCredentials), 0o600); err != nil {
			return nil, nil, err
		}
		gcpEnv = append(gcpEnv, fmt.Sprintf("GOOGLE_APPLICATION_CREDENTIALS=%s", path.Join(tempDir, "creds.json")))
	}

	return gcpArgs, gcpEnv, nil
}

func (cl *ByocClient) generateByocArgsAndEnv(cluster cloudapi.Cluster, verb string) (args, env []string, err error) {
	cloudProvider := strings.ToLower(cluster.Spec.Provider)
	byocArgs := []string{
		cloudProvider, verb,
		"--cloud-api-token", cl.authToken,
		"--redpanda-id", cluster.ID,
		"--debug",
	}
	byocEnv := []string{
		fmt.Sprintf("CLOUD_URL=%s/api/v1", cl.internalAPIURL),
	}
	for _, s := range os.Environ() {
		// include all current environment variables, except for Terraform variables
		// that byoc doesn't like and that might mess up the Terraform process that
		// byoc calls.
		if !strings.HasPrefix(s, "TF_") {
			byocEnv = append(byocEnv, s)
		}
	}

	providerArgs, providerEnv, err := func() ([]string, []string, error) {
		switch cloudProvider {
		case CloudProviderStringAws:
			return cl.generateAwsArgsAndEnv()
		case CloudProviderStringAzure:
			return cl.generateAzureArgsAndEnv()
		case CloudProviderStringGcp:
			return cl.generateGcpArgsAndEnv()
		default:
			return nil, nil, fmt.Errorf(
				"unimplemented cloud provider %v. please report this issue to the provider developers",
				cloudProvider,
			)
		}
	}()
	if err != nil {
		return nil, nil, err
	}
	byocArgs = append(byocArgs, providerArgs...)
	byocEnv = append(byocEnv, providerEnv...)

	return byocArgs, byocEnv, nil
}

func (cl *ByocClient) getByocExecutable(ctx context.Context, cluster cloudapi.Cluster) (string, error) {
	// TODO: try to cache this in local directory somewhere. beware race conditions.
	// TODO: grab the existing one from rpk if it has the correct checksum?

	pack, err := cl.api.InstallPack(ctx, cluster.Spec.InstallPackVersion)
	if err != nil {
		return "", fmt.Errorf("unable to request install pack details for %q: %v",
			cluster.Spec.InstallPackVersion, err)
	}
	name := fmt.Sprintf("byoc-%s-%s", runtime.GOOS, runtime.GOARCH)
	artifact, found := pack.Artifacts.Find(name)
	if !found {
		return "", fmt.Errorf("unable to find byoc plugin %s in install pack", name)
	}

	tempDir, err := os.MkdirTemp("", "terraform-provider-redpanda")
	if err != nil {
		return "", err
	}
	// TODO: delete temp directory after running?

	// I'm reluctant to use more code from rpk since it's not meant as a public API, but let's
	// use this because it presumably will handle any new compression types if they're added
	// TODO: is it an issue that this loads the whole thing into memory before writing it?
	tflog.Info(ctx, fmt.Sprintf("downloading byoc plugin from: %v", artifact.Location))
	raw, err := plugin.Download(ctx, artifact.Location, false, "")
	if err != nil {
		return "", err
	}
	byocPath := path.Join(tempDir, "byoc")
	tflog.Info(ctx, fmt.Sprintf("writing byoc plugin to: %v", byocPath))
	err = os.WriteFile(byocPath, raw, 0o500) // #nosec G306 -- yes we want it to be executable
	if err != nil {
		return "", fmt.Errorf("error writing byoc executable: %w", err)
	}
	return byocPath, nil
}

func runSubprocess(ctx context.Context, env []string, executable string, args ...string) error {
	tempDir, err := os.MkdirTemp("", "terraform-provider-redpanda-byoc")
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Env = env

	// switch to new temporary directory so we don't fill this one up,
	// or in case this one isn't writable
	cmd.Dir = tempDir

	lastLogs := &lastLogs{}

	// Set up stdout pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	go forwardLogs(ctx, stdout, lastLogs)

	// Set up stderr pipe
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	go forwardLogs(ctx, stderr, lastLogs)

	// Start the command
	if err := cmd.Start(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}

	// Wait for completion
	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Use the thread-safe method to get logs
		return fmt.Errorf("%w:\n%v", err, strings.Join(lastLogs.GetLines(), "\n"))
	}

	return nil
}

type zapLine struct {
	DateTime string
	Level    string
	Logger   string
	File     string
	Message  string
}

var colorRegex = regexp.MustCompile("\x1B\\[(\\d{1,3}(;\\d{1,2};?)?)?[mGK]")

func removeColor(line string) string {
	return colorRegex.ReplaceAllString(line, "")
}

func parseZapLog(line string) *zapLine {
	const n = 5
	parts := strings.SplitN(line, "\t", n)
	if len(parts) != n {
		return nil
	}
	return &zapLine{
		DateTime: parts[0],
		Level:    parts[1],
		Logger:   parts[2],
		File:     parts[3],
		Message:  parts[4],
	}
}

type lastLogs struct {
	Lines []string
	mutex sync.Mutex
}

func (l *lastLogs) Append(line string) {
	// 30 lines is enough to get any error that happens before rpk decides
	// to print the usage help
	// TODO: capture stderr lines like "Error: " or "failed " and then surface them when
	// the command fails instead of the whole log?
	const maxLines = 60
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if len(l.Lines) == maxLines {
		l.Lines = append(l.Lines[1:maxLines], line)
	} else {
		l.Lines = append(l.Lines, line)
	}
}

func (l *lastLogs) GetLines() []string {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	lines := make([]string, len(l.Lines))
	copy(lines, l.Lines)
	return lines
}

func forwardLogs(ctx context.Context, reader io.Reader, lastLogs *lastLogs) {
	r := bufio.NewScanner(reader)
	for {
		if !r.Scan() {
			return
		}
		line := r.Text()
		line = removeColor(line)
		lastLogs.Append(line)
		if z := parseZapLog(line); z != nil {
			switch z.Level {
			case "DEBUG", "INFO":
				tflog.Info(ctx, fmt.Sprintf("rpk: %s", z.Message))
			case "WARN":
				tflog.Warn(ctx, fmt.Sprintf("rpk: %s", z.Message))
			case "ERROR":
				tflog.Error(ctx, fmt.Sprintf("rpk: %s", z.Message))
			default:
				tflog.Info(ctx, fmt.Sprintf("rpk: %s\t%s", z.Level, z.Message))
			}
		} else {
			tflog.Info(ctx, fmt.Sprintf("rpk: %s", line))
		}
	}
}
