########################################
# Auto Scaling Group (ASG)
########################################
resource "aws_autoscaling_group" "this" {
  name                = "${var.environment}-asg"
  vpc_zone_identifier = var.subnet_ids
  min_size            = var.min_size
  max_size            = var.max_size
  desired_capacity    = var.desired_capacity

  launch_template {
    id      = var.launch_template_id
    version = var.launch_template_version
  }

  dynamic "tag" {
    for_each = var.asg_tags
    content {
      key                 = each.key
      value               = each.value
      propagate_at_launch = true
    }
  }
}

########################################
# Scaling Policy: Scale Up
########################################
resource "aws_autoscaling_policy" "scale_up" {
  name                   = "${var.environment}-scale-up"
  autoscaling_group_name = aws_autoscaling_group.this.name
  adjustment_type        = "ChangeInCapacity"
  scaling_adjustment     = var.scale_up_adjustment
  cooldown               = var.cooldown
}

########################################
# Scaling Policy: Scale Down
########################################
resource "aws_autoscaling_policy" "scale_down" {
  name                   = "${var.environment}-scale-down"
  autoscaling_group_name = aws_autoscaling_group.this.name
  adjustment_type        = "ChangeInCapacity"
  scaling_adjustment     = var.scale_down_adjustment
  cooldown               = var.cooldown
}

########################################
# CloudWatch Alarm: Scale Up
########################################
resource "aws_cloudwatch_metric_alarm" "scale_up_alarm" {
  alarm_name          = "${var.environment}-scale-up-alarm"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = var.evaluation_periods
  metric_name         = "CPUUtilization"
  namespace           = "AWS/EC2"
  period              = var.alarm_period
  statistic           = "Average"
  threshold           = var.scale_up_threshold
  alarm_description   = "Scale up if CPU utilization exceeds ${var.scale_up_threshold}%"
  dimensions = {
    AutoScalingGroupName = aws_autoscaling_group.this.name
  }
  alarm_actions = [aws_autoscaling_policy.scale_up.arn]
}

########################################
# CloudWatch Alarm: Scale Down
########################################
resource "aws_cloudwatch_metric_alarm" "scale_down_alarm" {
  alarm_name          = "${var.environment}-scale-down-alarm"
  comparison_operator = "LessThanThreshold"
  evaluation_periods  = var.evaluation_periods
  metric_name         = "CPUUtilization"
  namespace           = "AWS/EC2"
  period              = var.alarm_period
  statistic           = "Average"
  threshold           = var.scale_down_threshold
  alarm_description   = "Scale down if CPU utilization is less than ${var.scale_down_threshold}%"
  dimensions = {
    AutoScalingGroupName = aws_autoscaling_group.this.name
  }
  alarm_actions = [aws_autoscaling_policy.scale_down.arn]
}
