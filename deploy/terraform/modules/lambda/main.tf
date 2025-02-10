resource "aws_lambda_function" "sentiment_analysis" {
  function_name = "${var.environment}-sentiment-analysis"
  role          = var.iam_role_arn
  handler       = "main"
  runtime       = "go1.x"
  filename      = var.lambda_package_path
  publish       = true
  timeout       = 30

  vpc_config {
    subnet_ids         = [var.private_subnet_id]
    security_group_ids = [var.application_sg_id]
  }

  environment {
    variables = {
      OPENAI_API_KEY = var.openai_api_key
      DYNAMODB_TABLE = var.dynamodb_table_name
      # ... any other environment variables
    }
  }
}

