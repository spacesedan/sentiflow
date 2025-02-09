variable "environment" {
  type        = string
  description = "Environment name (e.g., dev, prod)."
}

variable "vpc_id" {
  type        = string
  description = "VPC ID where the instance will be created."
}

variable "private_subnet_id" {
  type        = string
  description = "Private subnet ID for Kafka instance."
}

variable "instance_type" {
  type        = string
  description = "EC2 instance type."
}

variable "ssh_key_name" {
  type        = string
  description = "SSH key for EC2 access."
}

variable "x_api_key" {
  type        = string
  description = "API key for x"
}

variable "kafka_bootstrap_servers" {
  type        = string
  description = "comma-separated kafka servers"
}
