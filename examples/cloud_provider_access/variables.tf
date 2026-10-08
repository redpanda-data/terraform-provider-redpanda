variable "cloud_provider_access_name" {
  default = "prod-aws-account"
}

variable "role_arn" {
  description = "ARN of the IAM role Redpanda assumes in your AWS account"
  default     = "arn:aws:iam::123456789012:role/redpanda-provisioner"
}
