output "table_name" {
  description = "Name of the DynamoDB table"
  value       = aws_dynamodb_table.tweets.name
}
