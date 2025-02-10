output "nlb_arn" {
  description = "The ARN of the Network Load Balancer."
  value       = aws_lb.this.arn
}

output "nlb_dns_name" {
  description = "The DNS name of the NLB."
  value       = aws_lb.this.dns_name
}

output "listener_arn" {
  description = "The ARN of the NLB listener."
  value       = aws_lb_listener.kafka_listener.arn
}

output "target_group_arn" {
  description = "The ARN of the target group (created by this module, if one was not provided)."
  value       = aws_lb_target_group.kafka_tg.arn
}
