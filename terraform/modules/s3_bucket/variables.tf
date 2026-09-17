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

variable "expiration_days" {
  description = "If greater than 0, objects are automatically deleted after this many days. 0 disables expiration."
  type        = number
  default     = 0
}
