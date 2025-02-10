data "aws_ami" "producer_ami" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["amzn2-ami-hvm-*-x86_64-gp2"]
  }
}

resource "aws_security_group" "producer_sg" {
  name   = "${var.environment}-producer-sg"
  vpc_id = var.vpc_id

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

data "template_file" "producer_userdata" {
  template = var.producer_user_data
}

resource "aws_launch_template" "producer_lt" {
  name_prefix            = "${var.environment}-producer-"
  image_id               = data.aws_ami.producer_ami.id
  instance_type          = var.instance_type
  key_name               = var.key_name
  vpc_security_group_ids = [aws_security_group.producer_sg.id]
  user_data              = base64encode(data.template_file.producer_userdata.rendered)
}

module "producer_autoscaling" {
  source = "../../modules/autoscaling/"

  autoscaling_group_name  = "producer-${var.environment}-autoscaling"
  environment             = var.environment
  launch_template_id      = aws_launch_template.producer_lt.id
  launch_template_version = "$Latest"
  subnet_ids              = var.subnet_ids
  min_size                = 1
  max_size                = 2
  desired_capacity        = 1

  asg_tags = {
    "Environment" = var.environment
    "Name"        = "${var.environment}-producer"
  }

  scale_up_threshold    = 75
  scale_down_threshold  = 30
  scale_up_adjustment   = 1
  scale_down_adjustment = -1
  alarm_period          = 300
  evaluation_periods    = 2
  cooldown              = 300
}
