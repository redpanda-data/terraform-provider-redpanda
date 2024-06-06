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

package tests

import (
	"context"
	"fmt"

	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
)

type sweepResourceGroup struct {
	ResourceGroupName string
	Client            *cloud.ControlPlaneClientSet
}

func (s sweepResourceGroup) SweepResourceGroup(_ string) error {
	ctx := context.Background()
	rg, err := s.Client.ResourceGroupForName(ctx, s.ResourceGroupName)
	if err != nil {
		return fmt.Errorf("unable to sweep resource group: unable to find resource group %q: %v", s.ResourceGroupName, err)
	}
	if _, err := s.Client.ResourceGroup.DeleteResourceGroup(ctx, &controlplanev1beta2.DeleteResourceGroupRequest{
		Id: rg.Name,
	}); err != nil {
		return fmt.Errorf("unable to sweep resource group: unable to delete resourceGroup %q: %v", s.ResourceGroupName, err)
	}
	return nil
}
