# Declare the action once. The cluster id comes from a variable rather than
# from redpanda_cluster.prod.id on purpose: a before_update trigger on the
# cluster runs ahead of the cluster, so an action config that reads the
# cluster's attributes forms a cycle and Terraform refuses the plan.
variable "prod_cluster_id" {
  type = string
}

action "redpanda_byoc_agent_apply" "prod" {
  config {
    cluster_id = var.prod_cluster_id
    timeout    = "45m"
  }
}

# Ad hoc, when Redpanda asks you to re-run the agent apply on an existing
# cluster (the equivalent of `rpk cloud byoc aws apply --redpanda-id=...`):
#
#   terraform apply -invoke=action.redpanda_byoc_agent_apply.prod
#
# Or apply the agent before every change to the cluster, so the cluster
# change runs through an up-to-date agent:
resource "redpanda_cluster" "prod" {
  name              = "prod"
  resource_group_id = redpanda_resource_group.prod.id
  network_id        = redpanda_network.prod.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  zones             = ["use1-az1", "use1-az2", "use1-az4"]
  throughput_tier   = "tier-1-aws-v3-arm"
  cluster_type      = "byoc"
  connection_type   = "private"

  lifecycle {
    action_trigger {
      events  = [before_update]
      actions = [action.redpanda_byoc_agent_apply.prod]
    }
  }
}
