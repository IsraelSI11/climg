// Package imagerecord defines the DynamoDB item shape shared by the Lambda
// (which writes it) and the CLI (which reads it). It lives under internal/
// so it can only be imported from within this module.
package imagerecord

import "fmt"

// Record is the DynamoDB item shape shared by the Lambda (which writes it
// once processing finishes) and the CLI (which reads it via status/list).
type Record struct {
	ImageID         string `dynamodbav:"image_id" json:"image_id"`
	RawBucket       string `dynamodbav:"raw_bucket" json:"raw_bucket"`
	RawKey          string `dynamodbav:"raw_key" json:"raw_key"`
	ProcessedBucket string `dynamodbav:"processed_bucket" json:"processed_bucket"`
	ProcessedKey    string `dynamodbav:"processed_key" json:"processed_key"`
	URL             string `dynamodbav:"url" json:"url"`
	Status          string `dynamodbav:"status" json:"status"`
	CreatedAt       string `dynamodbav:"created_at" json:"created_at"`
}

// PublicURL builds the virtual-hosted-style HTTPS URL for an object in a
// public S3 bucket. Both the Lambda (writing the record) and the CLI
// (predicting the URL before the Lambda has run) need this exact formula,
// so it lives here instead of being duplicated in each.
func PublicURL(region, bucket, key string) string {
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", bucket, region, key)
}
