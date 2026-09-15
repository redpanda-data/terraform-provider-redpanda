variable "cluster_api_url" {
  description = "Dataplane URL of an existing cluster, e.g. https://api-<id>.<rg>.byoc.prd.cloud.redpanda.com"
  type        = string
}

variable "user_name" {
  description = "Name of the user to create"
  type        = string
  default     = "example-user"
}
