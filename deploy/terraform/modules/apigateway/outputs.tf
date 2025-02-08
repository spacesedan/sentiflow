output "api_invoke_url" {
  description = "Invoke URL for the API Gateway"
  value       = "${aws_api_gateway_rest_api.tweets_api.execution_arn}/${var.environment}"
}
