variable "environment" {
  description = "The environment name"
  type        = string
}

variable "alert_email" {
  description = "Email address to receive SNS notifications for alarm"
  type        = string

}

variable "lambda_function_name" {
  description = "The name of the Lambda function to monitor"
  type        = string
}


variable "lambda_log_retention_days" {
  description = "Number of days to retain the Lambda log group"
  type        = number
  default     = 14
}

variable "lambda_error_threshold" {
  description = "Error threshold (in percent) for the Kafka EC2 instance alarm"
  type        = number
  default     = 1
}

