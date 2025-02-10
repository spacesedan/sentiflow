data "aws_ami" "producer_ami" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["amzn2-ami-hvm-*-x86_64-gp2"]
  }
}

resource "aws_security_group" "producer_sg" {
  name        = "${var.environment}-producer-sg"
  description = "SG for the Tweet producer"
  vpc_id      = var.vpc_id

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_instance" "ec2_producer" {
  ami                    = data.aws_ami.producer_ami.id
  instance_type          = var.instance_type
  subnet_id              = var.private_subnet_id
  vpc_security_group_ids = [aws_security_group.producer_sg.id]
  key_name               = var.ssh_key_name
}
