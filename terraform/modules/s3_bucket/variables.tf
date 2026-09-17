variable "bucket_name" {
  description = "Globally unique name for the S3 bucket"
  type        = string
}

variable "tags" {
  description = "Tags applied to the bucket"
  type        = map(string)
  default     = {}
}

variable "public_read" {
  description = "Whether to allow public GetObject access to objects in this bucket"
  type        = bool
  default     = false
}
