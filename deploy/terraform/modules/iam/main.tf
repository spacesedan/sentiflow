resource "aws_iam_role" "execution_role" {
  name               = "execution_role"
  assume_role_policy = data.aws_iam_policy_document.lambda_trust.json
}

data "aws_iam_policy_document" "lambda_trust" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }

}


resource "aws_iam_policy_attachment" "lambda_basic_policy" {
  name       = "lambda_basic_policy"
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
  roles      = [aws_iam_role.execution_role.name]
}
