output "iam_role_arn" {
  description = "ARN of the IAM role"
  value       = aws_iam_role.execution_role.arn
}

output "iam_role_name" {
  description = "Name for the IAM role"
  value       = aws_iam_role.execution_role.name
}
