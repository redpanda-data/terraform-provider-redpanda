package utils

import "github.com/redpanda-data/terraform-provider-redpanda/redpanda/clients"

// ResourceData is used to pass data and dependencies to resource implementations
type ResourceData struct {
	CloudV2Client clients.CloudV2
}
