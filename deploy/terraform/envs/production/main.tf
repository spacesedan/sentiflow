
########################################
# Network Module (VPC, Subnets, etc.)
########################################
module "network" {
  source = "../../modules/network"

  vpc_cidr            = var.vpc_cidr
  public_subnet_cidr  = var.public_subnet_cidr
  private_subnet_cidr = var.private_subnet_cidr
}

########################################
# IAM Module (Lambda Execution Role)
########################################
module "iam" {
  source = "../../modules/iam"

  iam_role_name = "${var.environment}-lambda-execution-role"
  # The module should attach AWSLambdaBasicExecutionRole, etc.
}

########################################
# Application Security Group
# (All resources that need to talk to Kafka join this SG)
########################################
resource "aws_security_group" "application_sg" {
  name        = "${var.environment}-application-sg"
  description = "Security group for app resources that talk to Kafka"
  vpc_id      = module.network.vpc_id

  # Typically, egress is open or restricted to Kafka ports if you want it stricter
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${var.environment}-application-sg"
  }
}

########################################
# Kafka Module (EC2 + SG for Kafka/Zookeeper)
########################################
module "kafka" {
  source = "../../modules/kafka"

  environment       = var.environment
  instance_type     = var.kafka_instance_type
  key_name          = var.ssh_key_name
  subnet_ids        = [module.network.private_subnet_id]
  application_sg_id = aws_security_group.application_sg.id
  kafka_user_data   = file("scripts/kafka_userdata.sh")
  vpc_id            = module.network.vpc_id

}

########################################
# Producer Module (Streams trending data to Kafka)
########################################
module "producer" {
  source = "../../modules/producer"

  environment        = var.environment
  instance_type      = var.producer_instance_type
  key_name           = var.ssh_key_name
  subnet_ids         = [module.network.private_subnet_id]
  producer_user_data = file("scripts/producer_userdata.sh")
  vpc_id             = module.network.vpc_id

}

########################################
# ALB Module (for Kafka traffic over TCP)
########################################
module "nlb" {
  source = "../../modules/nlb"

  environment       = var.environment
  vpc_id            = module.network.vpc_id
  public_subnet_ids = [module.network.public_subnet_id]
  application_sg_id = aws_security_group.application_sg.id
}

########################################
# DynamoDB Module (Tweets table)
########################################
module "dynamodb" {
  source = "../../modules/dynamodb"

  table_name = var.dynamodb_table_name
}

########################################
# Lambda Module (Sentiment Analysis)
########################################
module "lambda" {
  source              = "../../modules/lambda"
  environment         = var.environment
  private_subnet_id   = module.network.private_subnet_id
  iam_role_arn        = module.iam.iam_role_arn
  application_sg_id   = aws_security_group.application_sg.id
  lambda_package_path = var.lambda_package_path
  dynamodb_table_name = module.dynamodb.table_name
  openai_api_key      = var.openai_api_key
}

########################################
# API Gateway Module (REST API for tweets)
########################################
module "apigateway" {
  source               = "../../modules/apigateway"
  environment          = var.environment
  lambda_invoke_arn    = module.lambda.lambda_function_arn
  lambda_function_name = module.lambda.lambda_function_name
}

########################################
# Monitoring Module
# (Checking for Kafka CPU Ultilization and Lambda Errors)
########################################
module "monitoring" {
  source = "../../modules/monitoring"

  environment               = var.environment
  alert_email               = var.alert_email
  lambda_function_name      = module.lambda.lambda_function_name
  lambda_log_retention_days = 14
  lambda_error_threshold    = 1

}

########################################
# Outputs (Optional)
########################################
output "vpc_id" {
  description = "VPC ID from the network module"
  value       = module.network.vpc_id
}


output "nlb_dns_name" {
  description = "DNS name of the ALB"
  value       = module.nlb.nlb_dns_name
}

output "lambda_function_arn" {
  description = "Lambda Function ARN"
  value       = module.lambda.lambda_function_arn
}

output "api_gateway_url" {
  description = "Base URL for the API Gateway"
  value       = module.apigateway.api_invoke_url
}
