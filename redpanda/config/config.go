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

// Package config contains the configuration structs to initialize our clients
// and provider.
package config

import (
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"google.golang.org/grpc"
)

// Resource is the config used to pass data and dependencies to resource
// implementations.
type Resource struct {
	AuthToken              string
	ByocClient             *utils.ByocClient
	ControlPlaneConnection *grpc.ClientConn
}

// Datasource is the config used to pass data and dependencies to data source
// implementations.
type Datasource struct {
	AuthToken              string
	ControlPlaneConnection *grpc.ClientConn
}

// TODO add cloud provider and region as values to persist
