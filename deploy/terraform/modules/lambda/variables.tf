variable "environment" {
  type        = string
  description = "Environment name."
}

variable "private_subnet_id" {
  type        = string
  description = "CIDR block for the private subnet"

}

variable "iam_role_arn" {
  type        = string
  description = "IAM Role ARN for the Lambda execution."
}

variable "application_sg_id" {
  type        = string
  description = "Security group ID for any application that needs Kafka access"
}

variable "openai_api_key" {
  type        = string
  description = "API key for OpenAI."
  sensitive   = true
}

variable "lambda_package_path" {
  type        = string
  description = "Path to the compiled Go binary zip for the Lambda function."
}

variable "dynamodb_table_name" {
  type        = string
  description = "Name of the DynamoDB table for tweets."
}
