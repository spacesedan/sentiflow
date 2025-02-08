output "alb_arn" {
  description = "ARN of the ALB."
  value       = aws_lb.this.arn
}

output "alb_dns_name" {
  description = "DNS name of the ALB."
  value       = aws_lb.this.dns_name
}

output "kafka_target_group_arn" {
  description = "ARN of the Kafka target group."
  value       = aws_lb_target_group.kafka_tg.arn
}
