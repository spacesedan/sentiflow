variable "environment" {
  description = "The environment name (e.g., production, dev)"
  type        = string
}

variable "application_sg_id" {
  type        = string
  description = "Security group ID for the application that needs Kafka access"
}


variable "instance_type" {
  description = "EC2 instance type for the Kafka broker"
  type        = string
  default     = "t3.micro"
}

variable "key_name" {
  description = "SSH key pair name for the Kafka broker"
  type        = string
}

variable "subnet_ids" {
  description = "List of subnet IDs for deploying Kafka"
  type        = list(string)
}

variable "kafka_user_data" {
  description = "User data script for the Kafka broker"
  type        = string
}

