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

package sweep

import (
	"context"
	"time"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// ServerlessPrivateLink sweeps a serverless private link by name.
type ServerlessPrivateLink struct {
	PrivateLinkName string
	Client          *cloud.ControlPlaneClientSet
}

// SweepServerlessPrivateLink deletes the serverless private link matching PrivateLinkName.
func (s ServerlessPrivateLink) SweepServerlessPrivateLink(_ string) error {
	ctx := context.Background()
	privateLink, err := s.Client.ServerlessPrivateLinkForName(ctx, s.PrivateLinkName)
	if err != nil {
		return err
	}

	op, err := s.Client.ServerlessPrivateLink.DeleteServerlessPrivateLink(ctx, &controlplanev1.DeleteServerlessPrivateLinkRequest{
		Id: privateLink.GetId(),
	})
	if err != nil {
		return err
	}

	return utils.AreWeDoneYet(ctx, op.Operation, 5*time.Minute, s.Client.Operation)
}
