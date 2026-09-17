output "raw_bucket_name" {
  description = "Name of the S3 bucket where original images are uploaded"
  value       = module.raw_bucket.bucket_name
}

output "aws_region" {
  value = var.aws_region
}

output "processed_bucket_name" {
  description = "Name of the public S3 bucket serving optimized WebP images"
  value       = module.processed_bucket.bucket_name
}

output "processed_bucket_domain" {
  description = "Regional domain to build public URLs: https://<domain>/<key>"
  value       = module.processed_bucket.bucket_regional_domain_name
}

output "dynamodb_table_name" {
  value = aws_dynamodb_table.images.name
}

output "lambda_function_name" {
  value = aws_lambda_function.image_processor.function_name
}
