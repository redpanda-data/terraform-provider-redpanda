provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = var.resource_group_name
}

resource "redpanda_serverless_cluster" "test" {
  name              = var.cluster_name
  resource_group_id = redpanda_resource_group.test.id
  serverless_region = var.region
  allow_deletion    = true

  networking_config = {
    public  = "STATE_ENABLED"
    private = "STATE_DISABLED"
  }
}

resource "redpanda_schema_registry_acl" "provider_bootstrap_subject" {
  cluster_id     = redpanda_serverless_cluster.test.id
  principal      = "User:*"
  resource_type  = "SUBJECT"
  resource_name  = var.resource_group_name
  pattern_type   = "PREFIXED"
  host           = "*"
  operation      = "ALL"
  permission     = "ALLOW"
  allow_deletion = true
}

resource "redpanda_schema_registry_acl" "provider_bootstrap_registry" {
  cluster_id     = redpanda_serverless_cluster.test.id
  principal      = "User:*"
  resource_type  = "REGISTRY"
  resource_name  = "*"
  pattern_type   = "LITERAL"
  host           = "*"
  operation      = "ALL"
  permission     = "ALLOW"
  allow_deletion = true
}

# Metadata-only changes (description/default) must register a new version.
resource "redpanda_schema" "json_doc_test" {
  cluster_id     = redpanda_serverless_cluster.test.id
  subject        = "${var.resource_group_name}-json-value"
  schema_type    = "JSON"
  compatibility  = "NONE"
  references     = []
  allow_deletion = true
  schema         = var.json_schema_definition

  depends_on = [
    redpanda_schema_registry_acl.provider_bootstrap_subject,
    redpanda_schema_registry_acl.provider_bootstrap_registry,
  ]
}

# Metadata-only changes (doc/default/aliases/order) must register a new
# version. compatibility = NONE so the body can vary freely.
resource "redpanda_schema" "avro_doc_test" {
  cluster_id     = redpanda_serverless_cluster.test.id
  subject        = "${var.resource_group_name}-avro-value"
  schema_type    = "AVRO"
  compatibility  = "NONE"
  references     = []
  allow_deletion = true
  schema         = var.avro_schema_definition

  depends_on = [
    redpanda_schema_registry_acl.provider_bootstrap_subject,
    redpanda_schema_registry_acl.provider_bootstrap_registry,
  ]
}

# Authored message-first with a package-relative type reference so Schema
# Registry canonicalizes it on write. compatibility = NONE lets the body change
# freely between steps. references = [] also exercises the empty-list fix.
resource "redpanda_schema" "protobuf_test" {
  cluster_id     = redpanda_serverless_cluster.test.id
  subject        = "${var.resource_group_name}-protobuf-value"
  schema_type    = "PROTOBUF"
  compatibility  = "NONE"
  references     = []
  allow_deletion = true
  schema         = var.protobuf_schema_definition

  depends_on = [
    redpanda_schema_registry_acl.provider_bootstrap_subject,
    redpanda_schema_registry_acl.provider_bootstrap_registry,
  ]
}
