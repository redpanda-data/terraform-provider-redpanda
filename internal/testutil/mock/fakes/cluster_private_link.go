// Copyright 2026 Redpanda Data, Inc.
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

package fakes

import (
	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
)

// The control plane attaches a status to every enabled private-link block
// (public-api-go cluster mapper, gated on spec.Enabled && status != nil). The
// connection lists stay nil until a customer endpoint connects; proto3 sends
// an empty repeated field as absent, so nil is what the provider reads. The
// console port is reported only while connect_console is true.

const fakePrivateLinkConsolePort = 443

func awsPrivateLinkStatus(connectConsole bool) *controlplanev1.Cluster_AWSPrivateLink_Status {
	st := &controlplanev1.Cluster_AWSPrivateLink_Status{
		ServiceId:                 "vpce-svc-0123456789abcdef0",
		ServiceName:               "com.amazonaws.vpce.us-east-1.vpce-svc-0123456789abcdef0",
		ServiceState:              "Available",
		KafkaApiSeedPort:          30292,
		SchemaRegistrySeedPort:    30081,
		RedpandaProxySeedPort:     30282,
		KafkaApiNodeBasePort:      32092,
		RedpandaProxyNodeBasePort: 35082,
	}
	if connectConsole {
		st.ConsolePort = fakePrivateLinkConsolePort
	}
	return st
}

func gcpPrivateServiceConnectStatus() *controlplanev1.Cluster_GCPPrivateServiceConnect_Status {
	return &controlplanev1.Cluster_GCPPrivateServiceConnect_Status{
		ServiceAttachment:         "projects/tfrp-fake/regions/us-central1/serviceAttachments/tfrp-fake",
		KafkaApiSeedPort:          30292,
		SchemaRegistrySeedPort:    30081,
		RedpandaProxySeedPort:     30282,
		KafkaApiNodeBasePort:      32092,
		RedpandaProxyNodeBasePort: 35082,
	}
}

func azurePrivateLinkStatus(connectConsole bool) *controlplanev1.Cluster_AzurePrivateLink_Status {
	st := &controlplanev1.Cluster_AzurePrivateLink_Status{
		ServiceId:                 "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/tfrp-fake/providers/Microsoft.Network/privateLinkServices/tfrp-fake",
		ServiceName:               "tfrp-fake",
		DnsARecord:                "10.0.0.4",
		KafkaApiSeedPort:          30292,
		SchemaRegistrySeedPort:    30081,
		RedpandaProxySeedPort:     30282,
		KafkaApiNodeBasePort:      32092,
		RedpandaProxyNodeBasePort: 35082,
	}
	if connectConsole {
		st.ConsolePort = fakePrivateLinkConsolePort
	}
	return st
}
