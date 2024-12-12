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
	"encoding/base64"
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
	AuthToken               string
	AzureSubscriptionID     string
	GcpProject              string
	InternalAPIURL          string
	AzureClientID           string
	AzureClientSecret       string
	AzureTenantID           string
	GoogleCredentials       string
	GoogleCredentialsBase64 string
}

// ByocClient holds the information and clients needed to download and interact
// with the rpk byoc plugin.
type ByocClient struct {
	api                     *cloudapi.Client
	authToken               string
	internalAPIURL          string
	gcpProject              string
	azureSubscriptionID     string
	azureClientID           string
	azureClientSecret       string
	azureTenantID           string
	googleCredentials       string
	googleCredentialsBase64 string
}

// NewByocClient creates a new ByocClient.
func NewByocClient(conf ByocClientConfig) *ByocClient {
	return &ByocClient{
		api:                     cloudapi.NewClient(conf.InternalAPIURL, conf.AuthToken),
		authToken:               conf.AuthToken,
		internalAPIURL:          conf.InternalAPIURL,
		gcpProject:              conf.GcpProject,
		azureSubscriptionID:     conf.AzureSubscriptionID,
		azureClientID:           conf.AzureClientID,
		azureClientSecret:       conf.AzureClientSecret,
		azureTenantID:           conf.AzureTenantID,
		googleCredentials:       conf.GoogleCredentials,
		googleCredentialsBase64: conf.GoogleCredentialsBase64,
	}
}

// RunByoc downloads and runs the rpk byoc plugin for a given cluster id and verb
// ("apply" or "destroy").
func (cl *ByocClient) RunByoc(ctx context.Context, clusterID, verb string) error {
	cluster, err := cl.api.Cluster(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("unable to request cluster details for %q: %w", clusterID, err)
	}

	byocArgs, err := cl.generateByocArgs(cluster, verb)
	if err != nil {
		return err
	}

	byocPath, err := cl.getByocExecutable(ctx, cluster)
	if err != nil {
		return err
	}

	return runSubprocess(ctx, cl.internalAPIURL, cl.googleCredentials, cl.googleCredentialsBase64, byocPath, byocArgs...)
}

func (cl *ByocClient) generateByocArgs(cluster cloudapi.Cluster, verb string) ([]string, error) {
	cloudProvider := strings.ToLower(cluster.Spec.Provider)
	byocArgs := []string{
		cloudProvider, verb,
		"--cloud-api-token", cl.authToken,
		"--redpanda-id", cluster.ID, "--debug",
	}
	switch cloudProvider {
	case CloudProviderStringAws:
		// pass
	case CloudProviderStringAzure:
		if cl.azureSubscriptionID == "" {
			return nil, fmt.Errorf("value must be set for Azure Subscription ID")
		}
		byocArgs = append(byocArgs,
			"--subscription-id", cl.azureSubscriptionID,
			"--credential-source", "env",
			"--identity", "oidc",
			"--client-id", cl.azureClientID,
			"--client-secret", cl.azureClientSecret)
	case CloudProviderStringGcp:
		if cl.gcpProject == "" {
			return nil, fmt.Errorf("value must be set for GCP Project")
		}
		byocArgs = append(byocArgs, "--project-id", cl.gcpProject)
	default:
		return nil, fmt.Errorf(
			"unimplemented cloud provider %v. please report this issue to the provider developers",
			cloudProvider,
		)
	}
	return byocArgs, nil
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

func runSubprocess(ctx context.Context, cloudURL, gcreds, gcreds64, executable string, args ...string) error {
	// TODO: pass TF_LOG=JSON and parse message out?

	tempDir, err := os.MkdirTemp("", "terraform-provider-redpanda-byoc")
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, executable, args...)

	// switch to new temporary directory so we don't fill this one up,
	// or in case this one isn't writable
	cmd.Dir = tempDir

	// get rid of all pesky Terraform environment variables that byoc
	// doesn't like and that might mess up the Terraform process
	// that byoc calls
	cmd.Env = []string{}
	for _, s := range os.Environ() {
		if !strings.HasPrefix(s, "TF_") {
			cmd.Env = append(cmd.Env, s)
		}
	}
	cmd.Env = append(cmd.Env, fmt.Sprintf("CLOUD_URL=%s/api/v1", cloudURL))
	if gcreds != "" {
		if err := os.WriteFile(path.Join(tempDir, "creds.json"), []byte(gcreds), 0o600); err != nil {
			return err
		}
		cmd.Env = append(cmd.Env, fmt.Sprintf("GOOGLE_APPLICATION_CREDENTIALS=%s", path.Join(tempDir, "creds.json")))
	}
	if gcreds64 != "" {
		decodedBytes, err := base64.StdEncoding.DecodeString(gcreds64)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path.Join(tempDir, "creds.json"), decodedBytes, 0o600); err != nil {
			return err
		}
		cmd.Env = append(cmd.Env, fmt.Sprintf("GOOGLE_APPLICATION_CREDENTIALS=%s", path.Join(tempDir, "creds.json")))
	}
	lastLogs := &lastLogs{}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	go forwardLogs(ctx, stdout, lastLogs)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	go forwardLogs(ctx, stderr, lastLogs)

	err = cmd.Start()
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	err = cmd.Wait()
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%w:\n%v", err, strings.Join(lastLogs.Lines, "\n"))
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
	const n = 30
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if len(l.Lines) == n {
		l.Lines = append(l.Lines[1:n], line)
	} else {
		l.Lines = append(l.Lines, line)
	}
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
