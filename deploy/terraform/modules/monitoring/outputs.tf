output "lambda_log_group_name" {
  description = "The name of the CloudWatch log group for the Lambda function"
  value       = aws_cloudwatch_log_group.lambda_log_group.name
}

output "lambda_errors_alarm_arn" {
  description = "The ARN of the Lambda errors alarm"
  value       = aws_cloudwatch_metric_alarm.lambda_errors_alarm.arn
}

output "kafka_cpu_alarm_arn" {
  description = "The ARN of the Kafka CPU alarm"
  value       = aws_cloudwatch_metric_alarm.kafka_cpu_alarm.arn
}
