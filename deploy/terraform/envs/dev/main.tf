###############################################################################
# NETWORK MODULE
# (VPC, subnets, route, tables, etc.)
###############################################################################
module "network" {
  source              = "../../modules/network"
  vpc_cidr            = var.vpc_cidr
  public_subnet_cidr  = var.public_subnet_cidr
  private_subnet_cidr = var.private_subnet_cidr
}


###############################################################################
# APPLICATION SECURITY GROUP
# (Allows communication to kafka)
###############################################################################
resource "aws_security_group" "application_sg" {
  name        = "${var.environment}-application-sg"
  description = "Security group for application (EC2, ECS, Lambda) that talk to Kafka"
  vpc_id      = module.network.vpc_id

  egress {
    from_port   = 9092
    to_port     = 9092
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${var.environment}-application-sg"
  }
}


###############################################################################
# IAM MODULE
# (Creates Lambda execution role with necessary policies)
###############################################################################
module "iam" {
  source = "../../modules/iam"

  iam_role_name = "${var.environment}-lambda-execution-role"
}

###############################################################################
# KAFKA MODULE
# (EC2 Insstance + security group for Kafka + Zookeeper)
###############################################################################
module "ec2_kafka" {
  source = "../../modules/ec2_kafka"

  vpc_id            = module.network.vpc_id
  private_subnet_id = module.network.private_subnet_id
  instance_type     = var.kafka_instance_type
  environment       = var.environment
  ssh_key_name      = var.ssh_key_name
  application_sg_id = aws_security_group.application_sg.id
}


###############################################################################
# LAMBDA MODULE
# (Sentiment analysis function, VPC-enabled for Kafka access)
###############################################################################
module "lambda" {
  source = "../../modules/lambda"

  environment         = var.environment
  application_sg_id   = aws_security_group.application_sg.id
  private_subnet_id   = module.network.public_subnet_id
  iam_role_arn        = module.iam.iam_role_arn
  lambda_package_path = "../../../../build/sentiment-analysis.zip"
  dynamodb_table_name = var.dynamodb_table_name
  openai_api_key      = var.openai_api_key
}

###############################################################################
# API GATEWAY MODULE
# (Exposes an endpoint for tweet senitment data)
###############################################################################
module "apigateway" {
  source = "../../modules/apigateway"

  environment          = var.environment
  lambda_function_name = module.lambda.lambda_function_name
  lambda_invoke_arn    = module.lambda.lambda_function_arn
}


###############################################################################
# API GATEWAY MODULE
# (Exposes an endpoint for tweet senitment data)
###############################################################################
module "dynamodb" {
  source = "../../modules/dynamodb"

  table_name = "tweets-dev"
}


###############################################################################
# OUTPUTS
###############################################################################
output "vpc_id" {
  description = "The ID of the created VPC from the network module"
  value       = module.network.vpc_id
}

output "kafka_instance_id" {
  description = "Kafka EC2 Instance ID"
  value       = module.ec2_kafka.kafka_instance_id
}

output "lambda_function_arn" {
  description = "The ARN of the Lambda function"
  value       = module.lambda.lambda_function_arn
}

output "api_gateway_url" {
  description = "API Gateway invoke URL"
  value       = module.apigateway.api_invoke_url
}

resource "aws_s3_bucket" "sentiflow_bucket" {
  bucket = var.s3_bucket_name
}


