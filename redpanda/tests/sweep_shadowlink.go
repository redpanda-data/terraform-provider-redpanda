// Copyright 2025 Redpanda Data, Inc.
//
//	Licensed under the Apache License, Version 2.0 (the "License");
//	you may not use this file except in compliance with the License.
//	You may obtain a copy of the License at
//
//	  http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS,
//	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//	See the License for the specific language governing permissions and
//	limitations under the License.

package tests

import (
	"context"
	"fmt"
	"time"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

type sweepShadowLink struct {
	LinkName        string
	ShadowClusterID string
	Client          *cloud.ControlPlaneClientSet
}

func (s sweepShadowLink) SweepShadowLinks(_ string) error {
	ctx := context.Background()
	req := &controlplanev1.ListShadowLinksRequest{}
	// Filter server-side when the cluster id is known; else list all and match by name.
	if s.ShadowClusterID != "" {
		req.Filter = &controlplanev1.ListShadowLinksRequest_Filter{
			ShadowRedpandaId: s.ShadowClusterID,
		}
	}
	resp, err := s.Client.ShadowLink.ListShadowLinks(ctx, req)
	if err != nil {
		return fmt.Errorf("unable to list shadow links for sweep: %v", err)
	}
	for _, item := range resp.GetShadowLinks() {
		if item.GetName() != s.LinkName {
			continue
		}
		op, err := s.Client.ShadowLink.DeleteShadowLink(ctx, &controlplanev1.DeleteShadowLinkRequest{Id: item.GetId()})
		if err != nil {
			if utils.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("unable to delete shadow link %q: %v", s.LinkName, err)
		}
		return utils.AreWeDoneYet(ctx, op.GetOperation(), 15*time.Minute, s.Client.Operation)
	}
	return nil
}
