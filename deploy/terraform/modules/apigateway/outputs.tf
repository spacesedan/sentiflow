# For convenience, provide an invoke URL for the stage
output "api_invoke_url" {
  description = "Invoke URL for the API Gateway stage."
  # This constructs a typical API Gateway base path
  value = "${aws_api_gateway_rest_api.tweets_api.execution_arn}/${var.environment}"
}

# If you prefer, you can build a more standard modern URL shape (some prefer partial references):
# or you can do something like:
# value = "https://${aws_api_gateway_rest_api.tweets_api.id}.execute-api.${var.region}.amazonaws.com/${var.environment}"

# You can also output the rest_api_id or stage_name if needed:
output "rest_api_id" {
  value = aws_api_gateway_rest_api.tweets_api.id
}

output "stage_name" {
  value = aws_api_gateway_stage.tweets_stage.stage_name
}
