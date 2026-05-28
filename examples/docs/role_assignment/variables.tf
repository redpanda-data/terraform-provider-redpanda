variable "example_password" {
  description = "Example user password. In real configs, source this from a secret store (TF_VAR_example_password, AWS Secrets Manager, Vault, etc.) rather than committing a literal."
  type        = string
  sensitive   = true
}
