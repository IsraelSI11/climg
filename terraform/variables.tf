variable "aws_region" {
  description = "AWS region where climg infrastructure is provisioned"
  type        = string
  default     = "us-east-1"
}

variable "project_name" {
  description = "Prefix used to name all climg resources"
  type        = string
  default     = "climg"
}
