output "raw_bucket_name" {
  description = "Name of the S3 bucket where original images are uploaded"
  value       = module.raw_bucket.bucket_name
}

output "aws_region" {
  value = var.aws_region
}
