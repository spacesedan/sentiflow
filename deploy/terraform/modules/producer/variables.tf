variable "environment" {
  description = "The enviroment name"
  type        = string
}

variable "instance_type" {
  description = "EC2 instance type for the tweet producer"
  type        = string
  default     = "t3.micro"
}

variable "key_name" {
  description = "SSH key pair name for the tweet producer"
  type        = string
}

variable "subnet_ids" {
  description = "List of subnet IDs to deploy the producer"
  type        = list(string)
}

variable "producer_user_data" {
  description = "Userdata script for the tweet producer"
  type        = string
}
