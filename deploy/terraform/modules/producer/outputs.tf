
output "launch_template_id" {
  description = "The launch template ID for the tweet producer"
  value       = aws_launch_template.producer_lt.id
}

output "producer_asg_name" {
  description = "The Auto Scaling Group name for the tweet producer"
  value       = module.producer_autoscaling.asg_name
}
