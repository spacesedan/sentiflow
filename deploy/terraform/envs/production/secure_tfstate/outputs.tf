
output "tfstate_bucket_name" {
  description = "The name of the S3 bucket for Terraform state."
  value       = aws_s3_bucket.tfstate_bucket.bucket
}

output "tfstate_lock_table_name" {
  description = "The name of the DynamoDB table for state locking."
  value       = aws_dynamodb_table.tfstate_lock.name
}
