variable "environment" {
  type        = string
  description = "Environment name."
}

variable "application_sg_id" {
  type        = string
  description = "Security group ID for the application that needs Kafka access"
}

variable "vpc_id" {
  type        = string
  description = "VPC ID."
}

variable "public_subnet_ids" {
  type        = list(string)
  description = "List of public subnet IDs for the ALB."
}

variable "kafka_instance_id" {
  type        = string
  description = "EC2 Instance ID for Kafka."
}
