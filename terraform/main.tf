data "aws_caller_identity" "current" {}

module "raw_bucket" {
  source      = "./modules/s3_bucket"
  bucket_name = "${var.project_name}-raw-${data.aws_caller_identity.current.account_id}"

  tags = {
    Project = var.project_name
    Purpose = "raw-uploads"
  }
}

# module "processed_bucket" se añade cuando lleguemos a la parte de la Lambda/WebP
