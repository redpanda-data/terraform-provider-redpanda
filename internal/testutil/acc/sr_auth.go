// Copyright 2026 Redpanda Data, Inc.
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

package acc

import (
	"context"
	"errors"
	"fmt"

	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/kclients"
)

// VerifySRAuth verifies that the user's password was correctly set by
// attempting to authenticate against Schema Registry. If password_wo was
// silently null (the bug), the SR will reject the request with 401.
func VerifySRAuth(ctx context.Context, cpCl *cloud.ControlPlaneClientSet, clusterName, username, password string) error {
	cluster, err := cpCl.ClusterForName(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("failed to look up cluster by name: %w", err)
	}

	// Empty authToken so the auth-precedence helper falls through to Basic;
	// this helper exists to verify the username+password path was wired
	// correctly.
	srClient, err := kclients.GetSchemaRegistryClientForCluster(ctx, cpCl, cluster.GetId(), "", username, password)
	if err != nil {
		return fmt.Errorf("failed to create SR client: %w", err)
	}

	types, err := srClient.SupportedTypes(ctx)
	if err != nil {
		return fmt.Errorf("SR auth failed — password_wo was likely not delivered to the API: %w", err)
	}
	if len(types) == 0 {
		return errors.New("SR should report at least one supported type, got none")
	}
	return nil
}
