variable "environment" {
  type        = string
  description = "Environment name."
}

variable "vpc_id" {
  type        = string
  description = "VPC ID."
}

variable "public_subnet_ids" {
  type        = list(string)
  description = "List of public subnet IDs for the ALB."
}

variable "alb_security_group_id" {
  type        = string
  description = "Security Group ID for the ALB."
}

variable "kafka_instance_id" {
  type        = string
  description = "EC2 Instance ID for Kafka."
}
