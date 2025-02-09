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

  ingress {
    description     = "Kafka"
    from_port       = 9092
    to_port         = 9092
    protocol        = "tcp"
    security_groups = [var.application_sg_id]
  }

  # Ingress not needed since zookeeper is runnign inside of this instance and 
  # no other service needs access to this port
  # ingress {
  #   description     = "Zookeeper"
  #   from_port       = 2181
  #   to_port         = 2181
  #   protocol        = "tcp"
  #   security_groups = [var.application_sg_id]
  # }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_instance" "kafka" {
  ami                    = data.aws_ami.kafka_ami.id
  instance_type          = var.instance_type
  subnet_id              = var.private_subnet_id
  vpc_security_group_ids = [aws_security_group.kafka_sg.id]
  key_name               = var.ssh_key_name

  user_data = file("${path.module}/install_kafka.sh")

  tags = {
    Name = "${var.environment}-kafka-ec2"
  }
}
