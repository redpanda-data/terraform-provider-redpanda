package utils

import "github.com/redpanda-data/terraform-provider-redpanda/redpanda/clients"

// DatasourceData is used to pass data and dependencies to data implementations
type DatasourceData struct {
	CloudV2Client clients.CloudV2
}
