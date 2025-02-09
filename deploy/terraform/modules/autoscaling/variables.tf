
variable "environment" {
  description = "The environment name (e.g., dev, production)."
  type        = string
}

variable "launch_template_id" {
  description = "The ID of the launch template to use for the ASG."
  type        = string
}

variable "launch_template_version" {
  description = "The version of the launch template to use."
  type        = string
  default     = "$Latest"
}

variable "subnet_ids" {
  description = "A list of subnet IDs to launch the instances in (preferably across multiple AZs)."
  type        = list(string)
}

variable "min_size" {
  description = "Minimum number of instances in the ASG."
  type        = number
  default     = 1
}

variable "max_size" {
  description = "Maximum number of instances in the ASG."
  type        = number
  default     = 2
}

variable "desired_capacity" {
  description = "Desired number of instances in the ASG."
  type        = number
  default     = 1
}

variable "asg_tags" {
  description = "A map of tags to assign to the ASG."
  type        = map(string)
  default     = {}
}

variable "scale_up_threshold" {
  description = "CPU utilization threshold (in percent) for scaling up."
  type        = number
  default     = 75
}

variable "scale_down_threshold" {
  description = "CPU utilization threshold (in percent) for scaling down."
  type        = number
  default     = 30
}

variable "scale_up_adjustment" {
  description = "How many instances to add when scaling up."
  type        = number
  default     = 1
}

variable "scale_down_adjustment" {
  description = "How many instances to remove when scaling down (use a negative number)."
  type        = number
  default     = -1
}

variable "alarm_period" {
  description = "The period (in seconds) for CloudWatch alarms."
  type        = number
  default     = 300
}

variable "evaluation_periods" {
  description = "Number of periods for evaluation in CloudWatch alarms."
  type        = number
  default     = 2
}

variable "cooldown" {
  description = "Cooldown period (in seconds) for scaling policies."
  type        = number
  default     = 300
}
