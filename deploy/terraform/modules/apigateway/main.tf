resource "aws_api_gateway_rest_api" "tweets_api" {
  name        = "${var.environment}-tweets_api"
  description = "API Gateway for tweet sentiment data"
}

resource "aws_api_gateway_resource" "tweets_resource" {
  rest_api_id = aws_api_gateway_rest_api.tweets_api.id
  parent_id   = aws_api_gateway_rest_api.tweets_api.root_resource_id
  path_part   = "tweets"
}

resource "aws_api_gateway_method" "tweets_get" {
  rest_api_id   = aws_api_gateway_rest_api.tweets_api.id
  resource_id   = aws_api_gateway_resource.tweets_resource.id
  http_method   = "GET"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "tweets_integration" {
  rest_api_id             = aws_api_gateway_rest_api.tweets_api.id
  resource_id             = aws_api_gateway_rest_api.tweets_api.root_resource_id
  http_method             = aws_api_gateway_method.tweets_get.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = var.lambda_invoke_arn
}

resource "aws_api_gateway_deployment" "tweets_deployment" {
  depends_on = [aws_api_gateway_integration.tweets_integration]

  rest_api_id = aws_api_gateway_rest_api.tweets_api.id
}

resource "aws_api_gateway_stage" "tweets_stage" {
  rest_api_id   = aws_api_gateway_rest_api.tweets_api.id
  deployment_id = aws_api_gateway_deployment.tweets_deployment.id
  stage_name    = var.environment
}

resource "aws_lambda_permission" "apigw_lambda" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = var.lambda_function_name
  principal     = "apigateway.amazonaws.com"

  source_arn = "${aws_api_gateway_rest_api.tweets_api.execution_arn}/*/*"
}
