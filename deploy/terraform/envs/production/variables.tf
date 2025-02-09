variable "environment" {
  type        = string
  description = "Environment name"
  default     = "production"
}

variable "aws_secret_key" {
  type        = string
  description = "AWS secret access key"
  sensitive   = true
}

variable "aws_access_key" {
  type        = string
  description = "AWS access key"
  sensitive   = true
}

variable "alert_email" {
  type        = string
  description = "Email used to send alerts to"
}


variable "vpc_cidr" {
  type    = string
  default = "10.10.0.0/16"
}

variable "public_subnet_cidr" {
  type    = string
  default = "10.10.1.0/24"
}

variable "private_subnet_cidr" {
  type    = string
  default = "10.10.2.0/24"
}


variable "kafka_instance_type" {
  type    = string
  default = "t3.micro"
}


variable "producer_instance_type" {
  type    = string
  default = "t3.micro"
}

variable "ssh_key_name" {
  type        = string
  description = "SSH key for Kafka EC2"
  default     = "sentiflow-prod-key"
}


variable "lambda_package_path" {
  type        = string
  description = "Path to the Lambda zip"
}


variable "openai_api_key" {
  type      = string
  sensitive = true
}

variable "dynamodb_table_name" {
  type    = string
  default = "tweets_production"
}

variable "x_api_key" {
  type      = string
  sensitive = true

}
