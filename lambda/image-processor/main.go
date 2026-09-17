package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// imageRecord is the item written to DynamoDB once an image has been
// converted to WebP. image_id is derived from the raw object's file name
// (without extension), so the CLI must upload originals as <uuid>.<ext>.
type imageRecord struct {
	ImageID         string `dynamodbav:"image_id"`
	RawBucket       string `dynamodbav:"raw_bucket"`
	RawKey          string `dynamodbav:"raw_key"`
	ProcessedBucket string `dynamodbav:"processed_bucket"`
	ProcessedKey    string `dynamodbav:"processed_key"`
	Status          string `dynamodbav:"status"`
	CreatedAt       string `dynamodbav:"created_at"`
}

var (
	s3Client        *s3.Client
	dynamoClient    *dynamodb.Client
	processedBucket = os.Getenv("PROCESSED_BUCKET")
	tableName       = os.Getenv("DYNAMODB_TABLE")
)

func main() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("unable to load AWS config: %v", err)
	}

	s3Client = s3.NewFromConfig(cfg)
	dynamoClient = dynamodb.NewFromConfig(cfg)

	lambda.Start(handleRequest)
}

func handleRequest(ctx context.Context, s3Event events.S3Event) error {
	for _, record := range s3Event.Records {
		if err := processRecord(ctx, record); err != nil {
			return fmt.Errorf("processing %s/%s: %w", record.S3.Bucket.Name, record.S3.Object.Key, err)
		}
	}
	return nil
}

func processRecord(ctx context.Context, record events.S3EventRecord) error {
	rawBucket := record.S3.Bucket.Name
	rawKey, err := url.QueryUnescape(record.S3.Object.Key)
	if err != nil {
		return fmt.Errorf("decoding object key: %w", err)
	}

	imageID := strings.TrimSuffix(path.Base(rawKey), path.Ext(rawKey))
	inputPath := "/tmp/" + path.Base(rawKey)
	outputPath := "/tmp/" + imageID + ".webp"
	processedKey := imageID + ".webp"

	if err := downloadObject(ctx, rawBucket, rawKey, inputPath); err != nil {
		return fmt.Errorf("downloading original: %w", err)
	}
	defer os.Remove(inputPath)

	if err := convertToWebP(inputPath, outputPath); err != nil {
		return fmt.Errorf("converting to webp: %w", err)
	}
	defer os.Remove(outputPath)

	if err := uploadObject(ctx, processedBucket, processedKey, outputPath); err != nil {
		return fmt.Errorf("uploading processed image: %w", err)
	}

	return saveMetadata(ctx, imageRecord{
		ImageID:         imageID,
		RawBucket:       rawBucket,
		RawKey:          rawKey,
		ProcessedBucket: processedBucket,
		ProcessedKey:    processedKey,
		Status:          "done",
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
	})
}

func downloadObject(ctx context.Context, bucket, key, destPath string) error {
	out, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}
	defer out.Body.Close()

	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, out.Body)
	return err
}

// convertToWebP shells out to the cwebp binary bundled via a Lambda layer
// at /opt/bin/cwebp (Lambda always adds /opt/bin to PATH).
func convertToWebP(inputPath, outputPath string) error {
	cmd := exec.Command("cwebp", "-quiet", "-q", "80", inputPath, "-o", outputPath)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func uploadObject(ctx context.Context, bucket, key, srcPath string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        f,
		ContentType: aws.String("image/webp"),
	})
	return err
}

func saveMetadata(ctx context.Context, rec imageRecord) error {
	item, err := attributevalue.MarshalMap(rec)
	if err != nil {
		return err
	}

	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})
	return err
}
