output "launch_template_id" {
  description = "The launch template ID for the Kafka broker"
  value       = aws_launch_template.kafka_lt.id
}

output "kafka_asg_name" {
  description = "The Auto Scaling Group name for the Kafka broker"
  value       = module.kafka_autoscaling.asg_name
}
