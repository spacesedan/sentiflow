variable "environment" {
  type        = string
  description = "Environment name (e.g., dev, prod)."
}

variable "application_sg_id" {
  type        = string
  description = "Security group ID for the application that needs Kafka access"
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
