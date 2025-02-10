provider "aws" {
  region = "us-west-2" # Adjust as needed
}

########################################
# S3 Bucket for Terraform State
########################################
resource "aws_s3_bucket" "tfstate_bucket" {
  bucket = "my-unique-tfstate-bucket-prod" # Must be globally unique

  versioning {
    enabled = true
  }

  tags = {
    Environment = "production"
    Name        = "TerraformStateBucket"
  }
}

########################################
# Public Access Block for the S3 Bucket
########################################
resource "aws_s3_bucket_public_access_block" "tfstate_bucket_public_access" {
  bucket = aws_s3_bucket.tfstate_bucket.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

########################################
# S3 Bucket Server-Side Encryption Configuration
########################################
resource "aws_s3_bucket_server_side_encryption_configuration" "tfstate_encryption" {
  bucket = aws_s3_bucket.tfstate_bucket.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

########################################
# DynamoDB Table for Terraform State Locking
########################################
resource "aws_dynamodb_table" "tfstate_lock" {
  name         = "tfstate-lock-prod"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "LockID"

  attribute {
    name = "LockID"
    type = "S"
  }

  tags = {
    Environment = "production"
    Name        = "TerraformStateLock"
  }
}
