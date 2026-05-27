variable "resource_group_name" {
  type        = string
  description = "Name of the resource group"
  default     = "example-resource-group"
}

variable "private_link_name" {
  type        = string
  description = "Name of the serverless private link"
  default     = "example-serverless-private-link"
}

variable "serverless_region" {
  type        = string
  description = "Redpanda serverless region. Valid values vary by environment — query `data \"redpanda_serverless_regions\" { cloud_provider = \"aws\" }` for the current list. The default works in production; in preprod use `eu-west-1`."
  default     = "pro-us-east-1"
}

variable "allowed_principals" {
  type        = list(string)
  description = "List of AWS principal ARNs allowed to connect to the private link"
  default     = ["arn:aws:iam::123456789012:root"]
}

variable "allow_deletion" {
  type        = bool
  description = "Allow deletion of the serverless private link. Set to true to allow terraform destroy."
  default     = false
}
