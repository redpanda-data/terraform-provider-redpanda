// Copyright 2024 Redpanda Data, Inc.
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

	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
)

type sweepGroup struct {
	GroupName string
	Client    *cloud.IAMClientSet
}

func (s sweepGroup) SweepGroup(_ string) error {
	ctx := context.Background()
	g, err := s.Client.GroupForName(ctx, s.GroupName)
	if err != nil {
		return fmt.Errorf("unable to sweep group: unable to find group %q: %v", s.GroupName, err)
	}
	if err := s.Client.DeleteGroup(ctx, g.GetId()); err != nil {
		return fmt.Errorf("unable to sweep group: unable to delete group %q: %v", s.GroupName, err)
	}
	return nil
}
