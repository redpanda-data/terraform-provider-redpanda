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

// Package models contains Terraform model definitions.
//
// Run `task generate` or `go generate ./redpanda/models/...` to generate all models.
package models

// ResourceGroup models
//go:generate go run ../../pkg/generate/modelgen -pkg=github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/resourcegroup -func=ResourceGroupSchema -type=resource -output=resourcegroup/resource_model_gen.go
//go:generate go run ../../pkg/generate/modelgen -pkg=github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/resourcegroup -func=DatasourceResourceGroupSchema -type=datasource -output=resourcegroup/data_model_gen.go

// Network models
//go:generate go run ../../pkg/generate/modelgen -pkg=github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/network -func=ResourceNetworkSchema -type=resource -ctx -output=network/resource_model_gen.go
//go:generate go run ../../pkg/generate/modelgen -pkg=github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/network -func=DatasourceNetworkSchema -type=datasource -output=network/data_model_gen.go

// Region models
//go:generate go run ../../pkg/generate/modelgen -pkg=github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/region -func=DataSourceRegionSchema -type=datasource -output=region/data_model_gen.go

// Role models
//go:generate go run ../../pkg/generate/modelgen -pkg=github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/role -func=ResourceRoleSchema -type=resource -output=role/resource_model_gen.go

// ServerlessCluster models
//go:generate go run ../../pkg/generate/modelgen -pkg=github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/serverlesscluster -func=ResourceServerlessClusterSchema -type=resource -output=serverlesscluster/resource_model_gen.go
//go:generate go run ../../pkg/generate/modelgen -pkg=github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/serverlesscluster -func=DatasourceServerlessClusterSchema -type=datasource -output=serverlesscluster/data_model_gen.go

// User models
//go:generate go run ../../pkg/generate/modelgen -pkg=github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/user -func=ResourceUserSchema -type=resource -output=user/resource_model_gen.go

// Topic models
//go:generate go run ../../pkg/generate/modelgen -pkg=github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/topic -func=ResourceTopicSchema -type=resource -output=topic/resource_model_gen.go

// ACL models
//go:generate go run ../../pkg/generate/modelgen -pkg=github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/acl -func=ResourceACLSchema -type=resource -output=acl/resource_model_gen.go

// SchemaRegistryACL models
//go:generate go run ../../pkg/generate/modelgen -pkg=github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/schemaregistryacl -func=ResourceSchemaRegistryACLSchema -type=resource -output=schemaregistryacl/resource_model_gen.go

// Schema models
//go:generate go run ../../pkg/generate/modelgen -pkg=github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/schema -func=ResourceSchemaSchema -type=resource -output=schema/resource_model_gen.go

// Cluster models
//go:generate go run ../../pkg/generate/modelgen -pkg=github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/cluster -func=ResourceClusterSchema -type=resource -ctx -output=cluster/resource_model_gen.go
//go:generate go run ../../pkg/generate/modelgen -pkg=github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/cluster -func=DatasourceClusterSchema -type=datasource -ctx -output=cluster/data_model_gen.go

// Serverless Private Link models
//go:generate go run ../../pkg/generate/modelgen -pkg=github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/serverlessprivatelink -func=ResourceServerlessPrivateLinkSchema -type=resource -output=serverlessprivatelink/resource_model_gen.go
