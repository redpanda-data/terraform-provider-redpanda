variable "cluster_id" {
  default = ""
}

variable "topic_config" {
  default = {
    "cleanup.policy"   = "compact"
    "flush.ms"         = 100
    "compression.type" = "snappy"
  }
}

variable "user_name" {
  default = "data-test-username"
}

variable "user_pw" {
  default = "password"
}

variable "mechanism" {
  default = "scram-sha-256"
}

variable "topic_name" {
  default = "data-test-topic"
}

variable "partition_count" {
  default = 3
}

variable "replication_factor" {
  default = 3
}

variable "user_allow_deletion" {
  description = "Allow deletion of user resource"
  type        = bool
  default     = true
}

variable "acl_allow_deletion" {
  description = "Allow deletion of ACL resources"
  type        = bool
  default     = true
}