// Copyright 2023 Redpanda Data, Inc.
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
	"cmp"
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
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils/enums"
	"golang.org/x/oauth2"
)

// ByocClientConfig represents the options that must be passed to NewByocClient.
type ByocClientConfig struct {
	TokenSource         oauth2.TokenSource
	AzureSubscriptionID string
	GcpProject          string
	InternalAPIURL      string
	AzureClientID       string
	AzureClientSecret   string
	AzureTenantID       string
	GoogleCredentials   string
	PublicAPIURL        string
	AwsAccessKeyID      string
	AwsSecretAccessKey  string
	AwsSessionToken     string
}

// ByocRunner runs the rpk byoc plugin against a cluster. *ByocClient is the
// production implementation; the integration tier substitutes a fake so the
// agent phases of Create and Delete run without a subprocess.
type ByocRunner interface {
	// RunByoc runs the plugin verb ("apply" or "destroy") for clusterID. When
	// sink is non-nil it receives the plugin's progress lines at INFO and
	// above, one call per line, in addition to the tflog forwarding.
	RunByoc(ctx context.Context, clusterID, verb string, sink func(line string)) error
}

// ByocCloudConfigChecker is the optional preflight a ByocRunner can offer:
// whether the provider configuration carries what the plugin needs for a
// cloud, checked before any download. Cloud provider strings are the
// enums.CloudProviderString* values.
type ByocCloudConfigChecker interface {
	CheckCloudConfig(cloudProvider string) error
}

var (
	_ ByocRunner             = (*ByocClient)(nil)
	_ ByocCloudConfigChecker = (*ByocClient)(nil)
)

// ByocClient holds the information and clients needed to download and interact
// with the rpk byoc plugin.
type ByocClient struct {
	ts                  oauth2.TokenSource
	internalAPIURL      string
	gcpProject          string
	azureSubscriptionID string
	azureClientID       string
	azureClientSecret   string
	azureTenantID       string
	googleCredentials   string
	publicAPIURL        string
	awsAccessKeyID      string
	awsSecretAccessKey  string
	awsSessionToken     string
}

// NewByocClient creates a new ByocClient.
func NewByocClient(conf ByocClientConfig) *ByocClient {
	return &ByocClient{
		ts:                  conf.TokenSource,
		internalAPIURL:      conf.InternalAPIURL,
		gcpProject:          conf.GcpProject,
		azureSubscriptionID: conf.AzureSubscriptionID,
		azureClientID:       conf.AzureClientID,
		azureClientSecret:   conf.AzureClientSecret,
		azureTenantID:       conf.AzureTenantID,
		googleCredentials:   conf.GoogleCredentials,
		publicAPIURL:        conf.PublicAPIURL,
		awsAccessKeyID:      conf.AwsAccessKeyID,
		awsSecretAccessKey:  conf.AwsSecretAccessKey,
		awsSessionToken:     conf.AwsSessionToken,
	}
}

// CheckCloudConfig implements ByocCloudConfigChecker with the same
// requirements generateByocArgsAndEnv enforces at run time. AWS is not
// checked: the plugin resolves AWS credentials from the ambient chain, which
// the provider cannot see.
func (cl *ByocClient) CheckCloudConfig(cloudProvider string) error {
	switch cloudProvider {
	case enums.CloudProviderStringGcp:
		if cl.gcpProject == "" {
			return errors.New("gcp_project_id must be set on the provider (or GOOGLE_PROJECT in the environment) to run the byoc plugin against a GCP cluster")
		}
	case enums.CloudProviderStringAzure:
		if cl.azureSubscriptionID == "" {
			return errors.New("azure_subscription_id must be set on the provider (or ARM_SUBSCRIPTION_ID in the environment) to run the byoc plugin against an Azure cluster")
		}
		if _, err := cl.resolveAzureAuth(); err != nil {
			return err
		}
	default:
	}
	return nil
}

// newAPI returns a fresh cloudapi.Client using a token fetched from the
// TokenSource. Each call hits the cache layer; the wire fetch is rare.
func (cl *ByocClient) newAPI() (*cloudapi.Client, error) {
	tok, err := cl.ts.Token()
	if err != nil {
		return nil, fmt.Errorf("acquire bearer token: %w", err)
	}
	return cloudapi.NewClient(cl.internalAPIURL, tok.AccessToken), nil
}

// RunByoc downloads and runs the rpk byoc plugin for a given cluster id and verb
// ("apply" or "destroy"). See ByocRunner for sink.
func (cl *ByocClient) RunByoc(ctx context.Context, clusterID, verb string, sink func(line string)) error {
	tflog.Info(ctx, "running byoc plugin", map[string]any{
		"cluster_id": clusterID,
		"verb":       verb,
	})
	api, err := cl.newAPI()
	if err != nil {
		return err
	}
	cluster, err := api.Cluster(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("unable to request cluster details for %q: %w", clusterID, err)
	}

	byocArgs, byocEnv, argsCleanup, err := cl.generateByocArgsAndEnv(ctx, cluster, verb)
	if err != nil {
		return err
	}
	if argsCleanup != nil {
		defer argsCleanup()
	}

	byocPath, execCleanup, err := cl.getByocExecutable(ctx, cluster)
	if err != nil {
		return err
	}
	defer execCleanup()

	return runSubprocess(ctx, byocEnv, sink, byocPath, byocArgs...)
}

func (cl *ByocClient) generateAwsArgsAndEnv() (args, env []string, err error) {
	var awsEnv []string
	if cl.awsAccessKeyID != "" {
		awsEnv = append(awsEnv, fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", cl.awsAccessKeyID))
	}
	if cl.awsSecretAccessKey != "" {
		awsEnv = append(awsEnv, fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", cl.awsSecretAccessKey))
	}
	if cl.awsSessionToken != "" {
		awsEnv = append(awsEnv, fmt.Sprintf("AWS_SESSION_TOKEN=%s", cl.awsSessionToken))
	}
	return nil, awsEnv, nil
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

// azureAuth is how the byoc plugin authenticates to Azure. credentialSource
// picks the credential for the plugin's own Azure clients, identity picks the
// azurerm login for its internal Terraform project, and an empty value leaves
// the plugin's default of cli.
type azureAuth struct {
	credentialSource string
	identity         string
	tokenFile        string
	rawToken         string
}

func (cl *ByocClient) hasAzureSecretOrCertificate() bool {
	return cl.azureClientSecret != "" || os.Getenv("ARM_CLIENT_CERTIFICATE") != "" || os.Getenv("ARM_CLIENT_CERTIFICATE_PATH") != ""
}

func azureFederatedTokenFile() string {
	return cmp.Or(os.Getenv("AZURE_FEDERATED_TOKEN_FILE"), os.Getenv("ARM_OIDC_TOKEN_FILE_PATH"))
}

// resolveAzureAuth maps the Terraform azurerm provider's environment onto the
// plugin's --credential-source and --identity flags. The plugin runs in two
// stages: pre-flight validation and its own Azure clients pick a credential
// from --credential-source=[cli|msi|env|workload], and the internal Terraform
// project sets use_msi, use_oidc, and use_cli from --identity=[cli|msi|oidc].
func (cl *ByocClient) resolveAzureAuth() (azureAuth, error) {
	authMethods := []struct {
		EnvName          string
		CredentialSource string
		Identity         string
	}{
		{"ARM_USE_MSI", "msi", "msi"},
		{"ARM_USE_OIDC", "", "oidc"},
		{"ARM_USE_CLI", "cli", "cli"},
		// --identity=none will set all of the other use_ configs to false.
		// Terraform will correctly pick up ARM_USE_AKS_WORKLOAD_IDENTITY from the environment.
		{"ARM_USE_AKS_WORKLOAD_IDENTITY", "workload", "none"},
	}
	var explicit *azureAuth
	for _, method := range authMethods {
		use, err := getEnvBoolean(method.EnvName)
		if err != nil {
			return azureAuth{}, err
		}
		if !use {
			continue
		}
		if explicit != nil {
			return azureAuth{}, errors.New("only one of ARM_USE_MSI, ARM_USE_OIDC, ARM_USE_CLI, or ARM_USE_AKS_WORKLOAD_IDENTITY can be set")
		}
		explicit = &azureAuth{credentialSource: method.CredentialSource, identity: method.Identity}
	}
	switch {
	case explicit != nil && explicit.identity == "oidc":
		return cl.resolveAzureOIDC()
	case explicit != nil:
		return *explicit, nil
	case cl.hasAzureSecretOrCertificate():
		// No --identity is sent. --identity=oidc would set use_oidc=true in the
		// provider config, which is not wanted, but it is also what passes
		// client_id and client_secret through to the backend config used after
		// the first-stage Terraform bootstrap. ARM_CLIENT_ID and
		// ARM_CLIENT_SECRET reach the backend without it; AZURE_CLIENT_ID and
		// AZURE_CLIENT_SECRET do not. --identity=none becomes possible once the
		// AZURE_ variables are no longer supported.
		return azureAuth{credentialSource: "env"}, nil
	case azureFederatedTokenFile() != "":
		return azureAuth{credentialSource: "workload", identity: "oidc", tokenFile: azureFederatedTokenFile()}, nil
	default:
		return azureAuth{}, nil
	}
}

// resolveAzureOIDC picks the plugin credential for ARM_USE_OIDC. The plugin's
// "env" source is azidentity's EnvironmentCredential, which cannot use a
// federated token, so a token goes through WorkloadIdentityCredential. With no
// token the caller is expected to have run az login, as azure/login does when
// azurerm fetches the GitHub Actions token itself.
func (cl *ByocClient) resolveAzureOIDC() (azureAuth, error) {
	if file := azureFederatedTokenFile(); file != "" {
		return azureAuth{credentialSource: "workload", identity: "oidc", tokenFile: file}, nil
	}
	if raw := os.Getenv("ARM_OIDC_TOKEN"); raw != "" {
		return azureAuth{credentialSource: "workload", identity: "oidc", rawToken: raw}, nil
	}
	if cl.hasAzureSecretOrCertificate() {
		return azureAuth{credentialSource: "env", identity: "oidc"}, nil
	}
	if _, err := exec.LookPath("az"); err != nil {
		return azureAuth{}, errors.New("ARM_USE_OIDC is set but no federated token was found and the Azure CLI is not installed: set AZURE_FEDERATED_TOKEN_FILE, ARM_OIDC_TOKEN_FILE_PATH, or ARM_OIDC_TOKEN, or run az login before Terraform")
	}
	return azureAuth{credentialSource: "cli", identity: "oidc"}, nil
}

// azureTokenCredentialName is the AZURE_TOKEN_CREDENTIALS value that confines
// azidentity's DefaultAzureCredential to the credential the plugin's own
// clients use. The plugin's tenant lookup, subnet provisioner, and
// resource-group cleanup ignore --credential-source and fall back to
// DefaultAzureCredential, the tenant lookup always and the others unless a
// client secret is used. Unconfined, that chain stops at
// ManagedIdentityCredential on a host with IMDS, such as a GitHub-hosted
// runner, before it reaches the Azure CLI. The env source needs no entry
// because EnvironmentCredential leads the chain.
func azureTokenCredentialName(credentialSource string) string {
	switch credentialSource {
	case "", "cli":
		return "AzureCLICredential"
	case "msi":
		return "ManagedIdentityCredential"
	case "workload":
		return "WorkloadIdentityCredential"
	default:
		return ""
	}
}

func (cl *ByocClient) generateAzureArgsAndEnv(ctx context.Context) (args, env []string, cleanup func(), err error) {
	auth, err := cl.resolveAzureAuth()
	if err != nil {
		return nil, nil, nil, err
	}
	// Command line arguments: pre-flight validation requires --subscription-id to be set, and
	// the internal Terraform project sets the subscription_id, client_id, and client_secret
	// provider variables based on --subscription-id, --client-id, and --client-secret.
	if cl.azureSubscriptionID == "" {
		return nil, nil, nil, errors.New("value must be set for Azure Subscription ID")
	}
	var azureArgs []string
	if auth.credentialSource != "" {
		azureArgs = append(azureArgs, "--credential-source", auth.credentialSource)
	}
	if auth.identity != "" {
		azureArgs = append(azureArgs, "--identity", auth.identity)
	}
	azureArgs = append(azureArgs,
		"--subscription-id", cl.azureSubscriptionID,
		"--client-id", cl.azureClientID,
		"--client-secret", cl.azureClientSecret,
	)
	// Without --tenant-id the plugin resolves the tenant through
	// DefaultAzureCredential before any other Azure call.
	if cl.azureTenantID != "" {
		azureArgs = append(azureArgs, "--tenant-id", cl.azureTenantID)
	}

	// Environment variables: pre-flight validation uses environment variables when passed
	// --credential-source=env, prefixed with AZURE_ instead of ARM_; and the internal
	// Terraform project sets the tenant_id provider variable based on AZURE_TENANT_ID.
	// Handle this by taking all the ARM_ environment variables used by the Terraform azurerm
	// provider and duplicate them as AZURE_ environment variables.
	var azureEnv []string
	for _, s := range os.Environ() {
		if strings.HasPrefix(s, "ARM_") {
			azureEnv = append(azureEnv, fmt.Sprintf("AZURE_%s", strings.TrimPrefix(s, "ARM_")))
		}
	}
	if cl.azureTenantID != "" {
		azureEnv = append(azureEnv, "AZURE_TENANT_ID="+cl.azureTenantID)
	}
	if cl.azureClientID != "" {
		azureEnv = append(azureEnv, "AZURE_CLIENT_ID="+cl.azureClientID)
	}
	// EnvironmentCredential reads the secret only from AZURE_CLIENT_SECRET,
	// and it must pair with the AZURE_CLIENT_ID exported above.
	if cl.azureClientSecret != "" {
		azureEnv = append(azureEnv, "AZURE_CLIENT_SECRET="+cl.azureClientSecret)
	}

	tokenFile := auth.tokenFile
	if auth.rawToken != "" {
		tokenFile, cleanup, err = writeAzureTokenFile(ctx, auth.rawToken)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	if tokenFile != "" {
		// WorkloadIdentityCredential reads AZURE_FEDERATED_TOKEN_FILE, while
		// azurerm under use_oidc reads ARM_OIDC_TOKEN_FILE_PATH.
		azureEnv = append(azureEnv,
			"AZURE_FEDERATED_TOKEN_FILE="+tokenFile,
			"ARM_OIDC_TOKEN_FILE_PATH="+tokenFile,
		)
	}
	if auth.identity == "oidc" {
		// The plugin's azurerm state backend gets no use_oidc setting and no
		// client_id without a secret, so it reads both from ARM_ variables and
		// otherwise falls back to the Azure CLI.
		azureEnv = append(azureEnv, "ARM_USE_OIDC=true")
		if cl.azureClientID != "" {
			azureEnv = append(azureEnv, "ARM_CLIENT_ID="+cl.azureClientID)
		}
	}
	if name := azureTokenCredentialName(auth.credentialSource); name != "" && os.Getenv("AZURE_TOKEN_CREDENTIALS") == "" {
		azureEnv = append(azureEnv, "AZURE_TOKEN_CREDENTIALS="+name)
	}
	return azureArgs, azureEnv, cleanup, nil
}

func writeAzureTokenFile(ctx context.Context, token string) (tokenFile string, cleanup func(), err error) {
	tempDir, err := os.MkdirTemp("", "terraform-provider-redpanda-azure")
	if err != nil {
		return "", nil, err
	}
	cleanup = func() {
		if err := os.RemoveAll(tempDir); err != nil {
			tflog.Warn(ctx, "failed to clean up Azure federated token temp directory", map[string]any{
				"path":  tempDir,
				"error": err.Error(),
			})
		}
	}
	tokenFile = path.Join(tempDir, "federated-token")
	if err := os.WriteFile(tokenFile, []byte(token), 0o600); err != nil {
		cleanup()
		return "", nil, err
	}
	return tokenFile, cleanup, nil
}

func (cl *ByocClient) generateGcpArgsAndEnv(ctx context.Context) (args, env []string, cleanup func(), err error) {
	if cl.gcpProject == "" {
		return nil, nil, nil, errors.New("value must be set for GCP Project")
	}
	gcpArgs := []string{
		"--project-id", cl.gcpProject,
	}

	// Handle credentials file creation.
	// Terraform's GCP provider accepts credentials directly in the GOOGLE_CREDENTIALS environment
	// variable, but rpk byoc does some pre-flight validation using a different library that
	// only allows credential paths passed in GOOGLE_APPLICATION_CREDENTIALS.
	gcpEnv := []string{}
	cleanup = func() {} // no-op by default
	if cl.googleCredentials != "" {
		tempDir, err := os.MkdirTemp("", "terraform-provider-redpanda-gcp")
		if err != nil {
			return nil, nil, nil, err
		}
		cleanup = func() {
			if err := os.RemoveAll(tempDir); err != nil {
				tflog.Warn(ctx, "failed to clean up GCP credentials temp directory", map[string]any{
					"path":  tempDir,
					"error": err.Error(),
				})
			}
		}
		if err := os.WriteFile(path.Join(tempDir, "creds.json"), []byte(cl.googleCredentials), 0o600); err != nil {
			cleanup() // Clean up on error
			return nil, nil, nil, err
		}
		gcpEnv = append(gcpEnv, fmt.Sprintf("GOOGLE_APPLICATION_CREDENTIALS=%s", path.Join(tempDir, "creds.json")))
	}

	return gcpArgs, gcpEnv, cleanup, nil
}

func (cl *ByocClient) generateByocArgsAndEnv(ctx context.Context, cluster cloudapi.Cluster, verb string) (args, env []string, cleanup func(), err error) {
	tok, err := cl.ts.Token()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("acquire bearer token: %w", err)
	}
	cloudProvider := strings.ToLower(cluster.Spec.Provider)
	byocArgs := []string{
		cloudProvider, verb,
		"--cloud-api-token", tok.AccessToken,
		"--redpanda-id", cluster.ID,
		"--debug",
	}
	byocEnv := []string{
		fmt.Sprintf("CLOUD_URL=%s/api/v1", cl.internalAPIURL),
		fmt.Sprintf("RPK_PUBLIC_API_URL=https://%s", strings.TrimSuffix(cl.publicAPIURL, ":443")),
	}
	for _, s := range os.Environ() {
		// include all current environment variables, except for Terraform variables
		// that byoc doesn't like and that might mess up the Terraform process that
		// byoc calls.
		if !strings.HasPrefix(s, "TF_") {
			byocEnv = append(byocEnv, s)
		}
	}

	var providerArgs, providerEnv []string
	var providerCleanup func()
	switch cloudProvider {
	case enums.CloudProviderStringAws:
		providerArgs, providerEnv, err = cl.generateAwsArgsAndEnv()
	case enums.CloudProviderStringAzure:
		providerArgs, providerEnv, providerCleanup, err = cl.generateAzureArgsAndEnv(ctx)
	case enums.CloudProviderStringGcp:
		providerArgs, providerEnv, providerCleanup, err = cl.generateGcpArgsAndEnv(ctx)
	default:
		err = fmt.Errorf(
			"unimplemented cloud provider %v. please report this issue to the provider developers",
			cloudProvider,
		)
	}
	if err != nil {
		return nil, nil, nil, err
	}
	byocArgs = append(byocArgs, providerArgs...)
	byocEnv = append(byocEnv, providerEnv...)

	return byocArgs, byocEnv, providerCleanup, nil
}

func (cl *ByocClient) getByocExecutable(ctx context.Context, cluster cloudapi.Cluster) (byocPath string, cleanup func(), err error) {
	// TODO: try to cache this in local directory somewhere. beware race conditions.
	// TODO: grab the existing one from rpk if it has the correct checksum?

	api, err := cl.newAPI()
	if err != nil {
		return "", nil, err
	}
	pack, err := api.InstallPack(ctx, cluster.Spec.InstallPackVersion)
	if err != nil {
		return "", nil, fmt.Errorf("unable to request install pack details for %q: %v",
			cluster.Spec.InstallPackVersion, err)
	}
	name := fmt.Sprintf("byoc-%s-%s", runtime.GOOS, runtime.GOARCH)
	artifact, found := pack.Artifacts.Find(name)
	if !found {
		return "", nil, fmt.Errorf("unable to find byoc plugin %s in install pack", name)
	}

	tempDir, err := os.MkdirTemp("", "terraform-provider-redpanda")
	if err != nil {
		return "", nil, err
	}
	cleanup = func() {
		if err := os.RemoveAll(tempDir); err != nil {
			tflog.Warn(ctx, "failed to clean up temp directory", map[string]any{
				"path":  tempDir,
				"error": err.Error(),
			})
		}
	}

	// I'm reluctant to use more code from rpk since it's not meant as a public API, but let's
	// use this because it presumably will handle any new compression types if they're added
	// TODO: is it an issue that this loads the whole thing into memory before writing it?
	tflog.Info(ctx, fmt.Sprintf("downloading byoc plugin from: %v", artifact.Location))
	raw, err := plugin.Download(ctx, artifact.Location, false, "")
	if err != nil {
		cleanup() // Clean up on error
		return "", nil, err
	}
	byocPath = path.Join(tempDir, "byoc")
	tflog.Info(ctx, fmt.Sprintf("writing byoc plugin to: %v", byocPath))
	err = os.WriteFile(byocPath, raw, 0o500) // #nosec G306 -- yes we want it to be executable
	if err != nil {
		cleanup() // Clean up on error
		return "", nil, fmt.Errorf("error writing byoc executable: %w", err)
	}
	return byocPath, cleanup, nil
}

func runSubprocess(ctx context.Context, env []string, sink func(string), executable string, args ...string) error {
	tempDir, err := os.MkdirTemp("", "terraform-provider-redpanda-byoc")
	if err != nil {
		return err
	}
	defer func() {
		if rmErr := os.RemoveAll(tempDir); rmErr != nil {
			tflog.Warn(ctx, "failed to remove byoc temp dir", map[string]any{"dir": tempDir, "error": rmErr.Error()})
		}
	}()

	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Env = env

	// switch to new temporary directory so we don't fill this one up,
	// or in case this one isn't writable
	cmd.Dir = tempDir

	lastLogs := &lastLogs{}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	// Start the command before spawning the log readers so a Start failure
	// doesn't leak goroutines blocked on a pipe that never fills.
	if err := cmd.Start(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); forwardLogs(ctx, stdout, lastLogs, sink) }()
	go func() { defer wg.Done(); forwardLogs(ctx, stderr, lastLogs, sink) }()

	// Per exec.Cmd.StdoutPipe's contract, all reads from the pipes must
	// complete before Wait because Wait closes them on process exit.
	wg.Wait()

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
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

func forwardLogs(ctx context.Context, reader io.Reader, lastLogs *lastLogs, sink func(string)) {
	r := bufio.NewScanner(reader)
	for {
		if !r.Scan() {
			return
		}
		line := r.Text()
		line = removeColor(line)
		lastLogs.Append(line)
		if sink != nil {
			if msg, ok := progressLine(line); ok {
				sink(msg)
			}
		}
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

// progressLine reduces one plugin output line to what a practitioner should
// see in the apply log: the message of a zap line at INFO or above, or a
// non-zap line as is. DEBUG lines are dropped because the plugin runs with
// --debug and they would swamp the log; they still reach tflog.
func progressLine(line string) (string, bool) {
	z := parseZapLog(line)
	if z == nil {
		return line, line != ""
	}
	switch z.Level {
	case "INFO", "WARN", "ERROR":
		return z.Message, true
	default:
		return "", false
	}
}
