
terraform {
  backend "s3" {
    bucket         = "my-unique-tfstate-bucket-prod"     # The bucket created above
    key            = "envs/production/terraform.tfstate" # The path within the bucket for the state file
    region         = "us-east-1"
    encrypt        = true                # Ensure encryption is used
    dynamodb_table = "tfstate-lock-prod" # The lock table created above
  }
}
