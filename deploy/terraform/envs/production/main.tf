resource "aws_s3_bucket" "sentiflow_bucket" {
  bucket = var.s3_bucket_name
}
