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

package resources

// User resource schema + model + flatten/expand
//go:generate go run ../../cmd/schemagen -proto-pkg=redpanda/api/dataplane/v1 -message=CreateUserRequest_User -config=user/schema.yaml -func=ResourceUserSchema -type=resource -output=user/schema_resource_gen.go -package=user -model-output=../models/user/resource_model_gen.go -model-package=user -conv-output=../models/user/conv_gen.go -proto-import=buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1 -proto-alias=dataplanev1

// Cluster resource schema + model + flatten/expand
//go:generate go run ../../cmd/schemagen -proto-pkg=redpanda/api/controlplane/v1 -message=Cluster -config=cluster/schema.yaml -func=ResourceClusterSchema -type=resource -output=cluster/schema_resource_gen.go -package=cluster -model-output=../models/cluster/resource_model_gen.go -model-package=cluster -conv-output=../models/cluster/conv_gen.go -proto-import=buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1 -proto-alias=controlplanev1

// Cluster datasource schema + model + flatten
//go:generate go run ../../cmd/schemagen -proto-pkg=redpanda/api/controlplane/v1 -message=Cluster -config=cluster/schema_datasource.yaml -func=DatasourceClusterSchema -type=datasource -output=cluster/schema_datasource_gen.go -package=cluster -model-output=../models/cluster/data_model_gen.go -model-package=cluster -conv-output=../models/cluster/data_conv_gen.go -proto-import=buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1 -proto-alias=controlplanev1

// Region datasource schema + model + flatten
//go:generate go run ../../cmd/schemagen -proto-pkg=redpanda/api/controlplane/v1 -message=Region -config=region/schema_datasource.yaml -func=DataSourceRegionSchema -type=datasource -output=region/schema_datasource_gen.go -package=region -model-output=../models/region/data_model_gen.go -model-package=region -conv-output=../models/region/conv_gen.go -proto-import=buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1 -proto-alias=controlplanev1

// ResourceGroup resource schema + model + flatten/expand
//go:generate go run ../../cmd/schemagen -proto-pkg=redpanda/api/controlplane/v1 -message=ResourceGroup -config=resourcegroup/schema.yaml -func=ResourceGroupSchema -type=resource -output=resourcegroup/schema_resource_gen.go -package=resourcegroup -model-output=../models/resourcegroup/resource_model_gen.go -model-package=resourcegroup -conv-output=../models/resourcegroup/conv_gen.go -proto-import=buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1 -proto-alias=controlplanev1

// ResourceGroup datasource schema + model + flatten
//go:generate go run ../../cmd/schemagen -proto-pkg=redpanda/api/controlplane/v1 -message=ResourceGroup -config=resourcegroup/schema_datasource.yaml -func=DatasourceResourceGroupSchema -type=datasource -output=resourcegroup/schema_datasource_gen.go -package=resourcegroup -model-output=../models/resourcegroup/data_model_gen.go -model-package=resourcegroup -conv-output=../models/resourcegroup/data_conv_gen.go -proto-import=buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1 -proto-alias=controlplanev1

// Role resource schema + model + flatten/expand
//go:generate go run ../../cmd/schemagen -proto-pkg=redpanda/api/dataplane/v1 -message=Role -config=role/schema.yaml -func=ResourceRoleSchema -type=resource -output=role/schema_resource_gen.go -package=role -model-output=../models/role/resource_model_gen.go -model-package=role -conv-output=../models/role/conv_gen.go -proto-import=buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1 -proto-alias=dataplanev1

// ACL resource schema + model + flatten/expand
//go:generate go run ../../cmd/schemagen -proto-pkg=redpanda/api/dataplane/v1 -message=CreateACLRequest -config=acl/schema.yaml -func=ResourceACLSchema -type=resource -output=acl/schema_resource_gen.go -package=acl -model-output=../models/acl/resource_model_gen.go -model-package=acl -conv-output=../models/acl/conv_gen.go -proto-import=buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1 -proto-alias=dataplanev1

// Topic resource schema + model + flatten/expand
//go:generate go run ../../cmd/schemagen -proto-pkg=redpanda/api/dataplane/v1 -message=CreateTopicRequest_Topic -config=topic/schema.yaml -func=ResourceTopicSchema -type=resource -output=topic/schema_resource_gen.go -package=topic -model-output=../models/topic/resource_model_gen.go -model-package=topic -conv-output=../models/topic/conv_gen.go -proto-import=buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1 -proto-alias=dataplanev1

// ServerlessPrivateLink resource schema + model + flatten/expand
//go:generate go run ../../cmd/schemagen -proto-pkg=redpanda/api/controlplane/v1 -message=ServerlessPrivateLink -config=serverlessprivatelink/schema.yaml -func=ResourceServerlessPrivateLinkSchema -type=resource -output=serverlessprivatelink/schema_resource_gen.go -package=serverlessprivatelink -model-output=../models/serverlessprivatelink/resource_model_gen.go -model-package=serverlessprivatelink -conv-output=../models/serverlessprivatelink/conv_gen.go -proto-import=buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1 -proto-alias=controlplanev1

// ServerlessCluster resource schema + model
//go:generate go run ../../cmd/schemagen -proto-pkg=redpanda/api/controlplane/v1 -message=ServerlessCluster -config=serverlesscluster/schema.yaml -func=ResourceServerlessClusterSchema -type=resource -output=serverlesscluster/schema_resource_gen.go -package=serverlesscluster -model-output=../models/serverlesscluster/resource_model_gen.go -model-package=serverlesscluster -conv-output=../models/serverlesscluster/conv_gen.go -proto-import=buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1 -proto-alias=controlplanev1

// ServerlessCluster datasource schema + model
//go:generate go run ../../cmd/schemagen -proto-pkg=redpanda/api/controlplane/v1 -message=ServerlessCluster -config=serverlesscluster/schema_datasource.yaml -func=DatasourceServerlessClusterSchema -type=datasource -output=serverlesscluster/schema_datasource_gen.go -package=serverlesscluster -model-output=../models/serverlesscluster/data_model_gen.go -model-package=serverlesscluster -conv-output=../models/serverlesscluster/data_conv_gen.go -proto-import=buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1 -proto-alias=controlplanev1

// Network resource schema + model
//go:generate go run ../../cmd/schemagen -proto-pkg=redpanda/api/controlplane/v1 -message=Network -config=network/schema.yaml -func=ResourceNetworkSchema -type=resource -output=network/schema_resource_gen.go -package=network -model-output=../models/network/resource_model_gen.go -model-package=network -conv-output=../models/network/conv_gen.go -proto-import=buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1 -proto-alias=controlplanev1

// Network datasource schema + model
//go:generate go run ../../cmd/schemagen -proto-pkg=redpanda/api/controlplane/v1 -message=Network -config=network/schema_datasource.yaml -func=DatasourceNetworkSchema -type=datasource -output=network/schema_datasource_gen.go -package=network -model-output=../models/network/data_model_gen.go -model-package=network -conv-output=../models/network/data_conv_gen.go -proto-import=buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1 -proto-alias=controlplanev1

// ShadowLink resource schema + model + flatten/expand
//go:generate go run ../../cmd/schemagen -proto-pkg=redpanda/api/controlplane/v1 -message=ShadowLink -config=shadowlink/schema.yaml -func=ResourceShadowLinkSchema -type=resource -output=shadowlink/schema_resource_gen.go -package=shadowlink -model-output=../models/shadowlink/resource_model_gen.go -model-package=shadowlink -conv-output=../models/shadowlink/conv_gen.go -proto-import=buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1 -proto-alias=controlplanev1

// Pipeline resource schema + model + flatten/expand
//go:generate go run ../../cmd/schemagen -proto-pkg=redpanda/api/dataplane/v1 -message=Pipeline -config=pipeline/schema.yaml -func=ResourcePipelineSchema -type=resource -output=pipeline/schema_resource_gen.go -package=pipeline -model-output=../models/pipeline/resource_model_gen.go -model-package=pipeline -conv-output=../models/pipeline/conv_gen.go -proto-import=buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1 -proto-alias=dataplanev1

// ServiceAccount resource schema + model + flatten/expand
//go:generate go run ../../cmd/schemagen -proto-pkg=redpanda/api/iam/v1 -message=ServiceAccount -config=serviceaccount/schema.yaml -func=ResourceServiceAccountSchema -type=resource -output=serviceaccount/schema_resource_gen.go -package=serviceaccount -model-output=../models/serviceaccount/resource_model_gen.go -model-package=serviceaccount -conv-output=../models/serviceaccount/conv_gen.go -proto-import=buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/iam/v1 -proto-alias=iamv1
