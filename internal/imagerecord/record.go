// Package imagerecord defines the DynamoDB item shape shared by the Lambda
// (which writes it) and the CLI (which reads it). It lives under internal/
// so it can only be imported from within this module.
package imagerecord

type Record struct {
	ImageID         string `dynamodbav:"image_id" json:"image_id"`
	RawBucket       string `dynamodbav:"raw_bucket" json:"raw_bucket"`
	RawKey          string `dynamodbav:"raw_key" json:"raw_key"`
	ProcessedBucket string `dynamodbav:"processed_bucket" json:"processed_bucket"`
	ProcessedKey    string `dynamodbav:"processed_key" json:"processed_key"`
	Status          string `dynamodbav:"status" json:"status"`
	CreatedAt       string `dynamodbav:"created_at" json:"created_at"`
}
