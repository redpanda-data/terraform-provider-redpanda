variable "resource_group_name" {
  default = "testname"
}

variable "network_name" {
  default = "testname"
}

variable "cluster_name" {
  default = "testname"
}

variable "region" {
  default = "us-east-2"
}

variable "zones" {
  default = ["use2-az1", "use2-az2", "use2-az3"]
}

variable "cloud_provider" {
  default = "aws"
}

variable "throughput_tier" {
  default = "tier-1-aws-v2-x86"
}

variable "connections" {
  description = "Listener topology applied to kafka_api, http_proxy, and schema_registry; the control plane requires all three to match."
  type = list(object({
    type = string
    auth = object({ mode = string })
  }))
  default = [
    { type = "public", auth = { mode = "sasl" } },
    { type = "private", auth = { mode = "sasl" } },
  ]
}
