resource "aws_dynamodb_table" "tweets" {
  name         = var.table_name
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "tweet_id"

  attribute {
    name = "tweet_id"
    type = "S"
  }

}
