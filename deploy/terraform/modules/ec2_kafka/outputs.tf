output "kafka_instance_id" {
  description = "Kafka EC2 instance ID."
  value       = aws_instance.kafka.id
}

output "kafka_security_group_id" {
  description = "Security group for the Kafka EC2 instance."
  value       = aws_security_group.kafka_sg.id
}

output "kafka_instance_private_dns" {
  description = "The private DNS of our Kafka instance"
  value       = aws_instance.kafka.private_dns
}
