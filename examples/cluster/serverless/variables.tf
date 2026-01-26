variable "resource_group_name" {
  default = "testgroup"
}

variable "cluster_name" {
  default = "testname"
}

variable "region" {
  default = "eu-west-1"
}

variable "topic_name" {
  default = "test-topic"
}

variable "partition_count" {
  default = 3
}

variable "replication_factor" {
  default = 3
}

variable "user_name" {
  default = "test-username"
}

variable "user_pw" {
  default = "password"
}

variable "mechanism" {
  default = "scram-sha-256"
}

variable "user_allow_deletion" {
  description = "Allow deletion of user resource"
  type        = bool
  default     = true
}

variable "public_networking" {
  description = "Value for public networking config (one of STATE_UNSPECIFIED, STATE_ENABLED, or STATE_DISABLED)"
  type = string
  default = "STATE_ENABLED"
}

variable "private_networking" {
  description = "Value for private networking config (one of STATE_UNSPECIFIED, STATE_ENABLED, or STATE_DISABLED)"
  type = string
  default = "STATE_DISABLED"
}

variable "allowed_principals" {
  description = "AWS principal ARNs allowed to connect via private link. Required when private_networking=STATE_ENABLED."
  type        = list(string)
  default     = []
}

variable "allow_private_link_deletion" {
  type        = bool
  description = "Allow deletion of the serverless private link. Set to true to allow terraform destroy."
  default     = false
}
