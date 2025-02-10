resource "aws_sns_topic" "alerts_topic" {
  name = "${var.environment}-alerts-topic"
}

resource "aws_sns_topic_subscription" "email_subscription" {
  topic_arn = aws_sns_topic.alerts_topic.arn
  protocol  = "email"
  endpoint  = var.alert_email

}

resource "aws_cloudwatch_log_group" "lambda_log_group" {
  name              = "/aws/lambda/${var.lambda_function_name}"
  retention_in_days = var.lambda_log_retention_days
}

resource "aws_cloudwatch_metric_alarm" "lambda_errors_alarm" {
  alarm_name          = "${var.environment}-lambda-errors"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 1
  metric_name         = "Errors"
  namespace           = "AWS/Lambda"
  period              = 300
  statistic           = "Sum"
  threshold           = var.lambda_error_threshold
  dimensions = {
    FunctionName = var.lambda_function_name
  }

  alarm_actions = [aws_sns_topic.alerts_topic.arn]
}

resource "aws_cloudwatch_metric_alarm" "kafka_cpu_alarm" {
  alarm_name          = "${var.environment}-kafka-cpu-high"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "CPUUtilization"
  namespace           = "AWS/EC2"
  period              = 300
  statistic           = "Average"
  threshold           = var.kafka_cpu_threshold
  alarm_description   = "Alarm when Kafka EC2 instance CPU utilization exceeps threshold"
  dimensions = {
    InstanceId = var.kafka_instance_id
  }

  alarm_actions = [aws_sns_topic.alerts_topic.arn]
}

resource "aws_cloudwatch_metric_alarm" "producer_cpu_alarm" {
  alarm_name          = "${var.environment}-kafka-cpu-high"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "CPUUtilization"
  namespace           = "AWS/EC2"
  period              = 300
  statistic           = "Average"
  threshold           = var.producer_instance_id
  alarm_description   = "Alarm when Kafka EC2 instance CPU utilization exceeps threshold"
  dimensions = {
    InstanceId = var.producer_instance_id
  }

  alarm_actions = [aws_sns_topic.alerts_topic.arn]
}
