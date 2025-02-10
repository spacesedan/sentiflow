resource "aws_security_group" "alb-sg" {
  name        = "${var.environment}-alb-sg"
  description = "Security group for the ALB"
  vpc_id      = var.vpc_id

  ingress {
    description = "Allow TCP 9092 inbound from anywhere (Kafka traffic)"
    from_port   = 9092
    to_port     = 9092
    protocol    = "tcp"
    # Best practice would be restricting this to only known IPs or subnets if possible
  }

  egress {
    from_port       = 0
    to_port         = 0
    protocol        = "-1"
    security_groups = [var.application_sg_id]
  }
}

resource "aws_lb" "this" {
  name               = "${var.environment}-kafka-alb"
  load_balancer_type = "network"
  subnets            = var.public_subnet_ids
  security_groups    = [aws_security_group.alb-sg.id]
}

resource "aws_lb_target_group" "kafka_tg" {
  name        = "${var.environment}-kafka-tg"
  port        = 9092
  protocol    = "TCP"
  vpc_id      = var.vpc_id
  target_type = "instance"
}

resource "aws_lb_listener" "kafka_listener" {
  load_balancer_arn = aws_lb.this.arn
  port              = "9092"
  protocol          = "TCP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.kafka_tg.arn
  }
}

resource "aws_lb_target_group_attachment" "kafka_tg_attachment" {
  target_group_arn = aws_lb_target_group.kafka_tg.arn
  target_id        = var.kafka_instance_id
  port             = 9092
}
