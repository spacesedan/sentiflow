output "producer_instance_id" {
  value       = aws_instance.ec2_producer.id
  description = "EC2 instance ID for the tweet producer"
}
