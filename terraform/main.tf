data "aws_caller_identity" "current" {}

module "raw_bucket" {
  source          = "./modules/s3_bucket"
  bucket_name     = "${var.project_name}-raw-${data.aws_caller_identity.current.account_id}"
  expiration_days = 30 # originals are only needed long enough for the Lambda to convert them

  tags = {
    Project = var.project_name
    Purpose = "raw-uploads"
  }
}

module "processed_bucket" {
  source      = "./modules/s3_bucket"
  bucket_name = "${var.project_name}-processed-${data.aws_caller_identity.current.account_id}"
  public_read = true

  tags = {
    Project = var.project_name
    Purpose = "processed-images"
  }
}

resource "aws_dynamodb_table" "images" {
  name           = "${var.project_name}-images"
  billing_mode   = "PROVISIONED"
  read_capacity  = 5
  write_capacity = 5
  hash_key       = "image_id"

  attribute {
    name = "image_id"
    type = "S"
  }

  tags = {
    Project = var.project_name
  }
}

data "aws_iam_policy_document" "lambda_assume_role" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "image_processor" {
  name               = "${var.project_name}-image-processor"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume_role.json
}

resource "aws_iam_role_policy_attachment" "lambda_logs" {
  role       = aws_iam_role.image_processor.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

data "aws_iam_policy_document" "lambda_permissions" {
  statement {
    sid       = "ReadRawBucket"
    actions   = ["s3:GetObject"]
    resources = ["${module.raw_bucket.bucket_arn}/*"]
  }

  statement {
    sid       = "WriteProcessedBucket"
    actions   = ["s3:PutObject"]
    resources = ["${module.processed_bucket.bucket_arn}/*"]
  }

  statement {
    sid       = "WriteImageMetadata"
    actions   = ["dynamodb:PutItem"]
    resources = [aws_dynamodb_table.images.arn]
  }
}

resource "aws_iam_role_policy" "image_processor" {
  name   = "${var.project_name}-image-processor-permissions"
  role   = aws_iam_role.image_processor.id
  policy = data.aws_iam_policy_document.lambda_permissions.json
}

resource "aws_lambda_layer_version" "cwebp" {
  filename            = "${path.module}/../lambda/layers/cwebp/build/layer.zip"
  layer_name          = "${var.project_name}-cwebp"
  compatible_runtimes = ["provided.al2023"]
  source_code_hash    = filebase64sha256("${path.module}/../lambda/layers/cwebp/build/layer.zip")
}

resource "aws_lambda_function" "image_processor" {
  function_name    = "${var.project_name}-image-processor"
  role             = aws_iam_role.image_processor.arn
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  architectures    = ["x86_64"]
  filename         = "${path.module}/../lambda/image-processor/build/function.zip"
  source_code_hash = filebase64sha256("${path.module}/../lambda/image-processor/build/function.zip")
  layers           = [aws_lambda_layer_version.cwebp.arn]
  timeout          = 30
  memory_size      = 512

  environment {
    variables = {
      PROCESSED_BUCKET = module.processed_bucket.bucket_name
      DYNAMODB_TABLE   = aws_dynamodb_table.images.name
    }
  }
}

resource "aws_lambda_permission" "allow_s3_invoke" {
  statement_id  = "AllowExecutionFromS3"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.image_processor.function_name
  principal     = "s3.amazonaws.com"
  source_arn    = module.raw_bucket.bucket_arn
}

resource "aws_s3_bucket_notification" "raw_upload" {
  bucket = module.raw_bucket.bucket_name

  lambda_function {
    lambda_function_arn = aws_lambda_function.image_processor.arn
    events              = ["s3:ObjectCreated:*"]
  }

  depends_on = [aws_lambda_permission.allow_s3_invoke]
}
