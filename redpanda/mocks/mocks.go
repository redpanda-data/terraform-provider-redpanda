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

//go:build generate
// +build generate

// Package mocks provides the mocked resources and its clients for testing.
package mocks

//go:generate mockgen -destination=./mock_topic_service_client.go -package=mocks buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/dataplane/v1alpha1/dataplanev1alpha1grpc TopicServiceClient
//go:generate mockgen -destination=./mock_user_service_client.go -package=mocks buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/dataplane/v1alpha1/dataplanev1alpha1grpc UserServiceClient
//go:generate mockgen -destination=./mock_operations_service_client.go -package=mocks buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1beta2/controlplanev1beta2grpc OperationServiceClient
//go:generate mockgen -destination=./mock_serverless_cluster_service_client.go -package=mocks buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1beta2/controlplanev1beta2grpc ServerlessClusterServiceClient
//go:generate mockgen -destination=./mock_cp_client_set.go -package=mocks github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud CpClientSet
