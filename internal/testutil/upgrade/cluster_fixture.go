//go:build upgrade

package upgrade

import (
	"os"
	"strings"
	"testing"
)

// ClusterFixture returns the Kafka cluster ID to use for upgrade tests
// that require a live cluster (TestUpgrade_User, TestUpgrade_Schema,
// TestUpgrade_Cluster, and the optional smoke suite). If KAFKA_CLUSTER_ID is
// set, it is returned directly (cluster-reuse path). Otherwise the test is
// skipped with an instruction to provision one.
func ClusterFixture(t *testing.T) string {
	t.Helper()
	if id := os.Getenv("KAFKA_CLUSTER_ID"); id != "" {
		return id
	}
	t.Skip("KAFKA_CLUSTER_ID not set; set to a live cluster ID to run cluster-dependent upgrade tests")
	return ""
}

// ClusterAPIURL returns the dataplane API URL for upgrade tests that require
// a live cluster (TestUpgrade_User). Reads KAFKA_CLUSTER_API_URL; skips if unset.
func ClusterAPIURL(t *testing.T) string {
	t.Helper()
	if url := os.Getenv("KAFKA_CLUSTER_API_URL"); url != "" {
		return url
	}
	t.Skip("KAFKA_CLUSTER_API_URL not set; set to a live cluster's dataplane API URL to run user upgrade tests")
	return ""
}

// ClusterAPIURLForms returns the legacy "host:443" and canonical "https://host"
// forms of the fixture dataplane URL, for cluster_api_url format-migration
// upgrade tests. Skips (via ClusterAPIURL) when KAFKA_CLUSTER_API_URL is unset.
func ClusterAPIURLForms(t *testing.T) (legacy, canonical string) {
	t.Helper()
	host := ClusterAPIURL(t)
	host = strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://")
	host = strings.TrimSuffix(strings.TrimSuffix(host, "/"), ":443")
	return host + ":443", "https://" + host
}
