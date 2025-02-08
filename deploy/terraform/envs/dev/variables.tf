variable "environment" {
  type        = string
  description = "Environment name"
  default     = "dev"
}

variable "aws_region" {
  description = "AWS region"
  default     = "us-west-2"
}

variable "vpc_cidr" {
  type        = string
  description = "CIDR for the VPC"
  default     = "10.0.0.0/16"
}

variable "public_subnet_cidr" {
  type        = string
  description = "CIDR for public subnet"
  default     = "10.0.1.0/24"
}

variable "private_subnet_cidr" {
  type        = string
  description = "CIDR for private subnet"
  default     = "10.0.2.0/24"
}

variable "kafka_instance_type" {
  type        = string
  description = "Instance type for Kafka EC2"
  default     = "t2.micro"
}

variable "ssh_key_name" {
  type        = string
  description = "EC2 ssh key for dev"
  default     = "my-dev-key"
}

variable "dynamodb_table_name" {
  type        = string
  description = "Name of the DynamoDB table"
  default     = "tweets_dev"
}

variable "openai_api_key" {
  type        = string
  description = "OpenAI API Key"
  sensitive   = true
}

variable "s3_bucket_name" {
  description = "S3 Bucket name"
  default     = "sentiflow-dev"
}
