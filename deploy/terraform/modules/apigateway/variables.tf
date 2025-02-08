variable "environment" {
  type        = string
  description = "Environment name."
}

variable "lambda_invoke_arn" {
  type        = string
  description = "Lambda function invoke ARN for API Gateway integration."
}

variable "lambda_function_name" {
  type        = string
  description = "Name of the Lambda function for API Gateway."
}
