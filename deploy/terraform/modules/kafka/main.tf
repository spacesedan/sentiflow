data "aws_ami" "kafka_ami" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["amzn2-ami-hvm-*-x86_64-gp2"]
  }
}


resource "aws_security_group" "kafka_sg" {
  name        = "${var.environment}-kafka-sg"
  description = "Security group for Kafka and Zookeeper"
  vpc_id      = var.vpc_id

  # Inbound for Kafka clients (port 9092)
  ingress {
    description     = "Allow Kafka traffic from application servers"
    from_port       = 9092
    to_port         = 9092
    protocol        = "tcp"
    security_groups = [var.application_sg_id] # Only allow traffic from the authorized application SG
  }

  # Inbound for Zookeeper (if needed)
  ingress {
    description     = "Allow Zookeeper traffic from application servers"
    from_port       = 2181
    to_port         = 2181
    protocol        = "tcp"
    security_groups = [var.application_sg_id]
  }

  # Egress: allow all outbound traffic (or restrict as needed)
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${var.environment}-kafka-sg"
  }
}

# Render the Kafka user data script
data "template_file" "kafka_userdata" {
  template = var.kafka_user_data
}

# Create a launch template for the Kafka broker
resource "aws_launch_template" "kafka_lt" {
  name_prefix            = "${var.environment}-kafka-"
  image_id               = data.aws_ami.kafka_ami.id
  instance_type          = var.instance_type
  key_name               = var.key_name
  vpc_security_group_ids = [aws_security_group.kafka_sg.id]
  user_data              = base64encode(data.template_file.kafka_userdata.rendered)
}

# Call the autoscaling module for Kafka (for demo, typically desired_capacity might be 1)
module "kafka_autoscaling" {
  source                  = "../modules/autoscaling"
  environment             = var.environment
  launch_template_id      = aws_launch_template.kafka_lt.id
  launch_template_version = "$Latest"
  subnet_ids              = var.subnet_ids
  min_size                = 1
  max_size                = 1
  desired_capacity        = 1

  asg_tags = {
    "Environment" = var.environment,
    "Name"        = "${var.environment}-kafka"
  }

  # These thresholds can be tuned as needed for your demo
  scale_up_threshold    = 80
  scale_down_threshold  = 50
  scale_up_adjustment   = 1
  scale_down_adjustment = -1
  alarm_period          = 300
  evaluation_periods    = 2
  cooldown              = 300
}

output "kafka_asg_name" {
  description = "The name of the Auto Scaling Group for the Kafka broker"
  value       = module.kafka_autoscaling.asg_name
}
