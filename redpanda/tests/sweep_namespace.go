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

	cloudv1beta1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/controlplane/v1beta1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

type sweepNamespace struct {
	NamespaceName string
	Client        cloudv1beta1.NamespaceServiceClient
}

func (s sweepNamespace) SweepNamespaces(_ string) error {
	ctx := context.Background()
	name, err := utils.FindNamespaceByName(ctx, s.NamespaceName, s.Client)
	if err != nil {
		return fmt.Errorf("unable to sweep namespace: unable to find namespace %q: %v", s.NamespaceName, err)
	}

	if _, err := s.Client.DeleteNamespace(ctx, &cloudv1beta1.DeleteNamespaceRequest{
		Id: name.GetId(),
	}); err != nil {
		return fmt.Errorf("unable to sweep namespace: unable to delete namespace %q: %v", s.NamespaceName, err)
	}
	return nil
}
