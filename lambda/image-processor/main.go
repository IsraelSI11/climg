// This Lambda is triggered by S3 ObjectCreated events on the raw uploads
// bucket. For each uploaded object it downloads the original, converts it
// to WebP via the bundled cwebp binary, uploads the result to the
// processed bucket, and records the outcome in DynamoDB.
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

	"github.com/IsraelSI11/climg/internal/imagerecord"
)

var (
	s3Client     *s3.Client
	dynamoClient *dynamodb.Client
	// PROCESSED_BUCKET and DYNAMODB_TABLE are injected by Terraform via the
	// Lambda's environment block (see aws_lambda_function.image_processor).
	processedBucket = os.Getenv("PROCESSED_BUCKET")
	tableName       = os.Getenv("DYNAMODB_TABLE")
)

// main runs once per execution environment, not once per invocation: the
// SDK clients are created here so every request handled by this warm
// environment reuses the same connections instead of reconnecting.
func main() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("unable to load AWS config: %v", err)
	}

	s3Client = s3.NewFromConfig(cfg)
	dynamoClient = dynamodb.NewFromConfig(cfg)

	lambda.Start(handleRequest)
}

// handleRequest is the entry point registered with lambda.Start. S3 can
// batch several object events into one invocation, so every record is
// processed; if any one fails, the whole invocation fails and Lambda
// retries the batch.
func handleRequest(ctx context.Context, s3Event events.S3Event) error {
	for _, record := range s3Event.Records {
		if err := processRecord(ctx, record); err != nil {
			return fmt.Errorf("processing %s/%s: %w", record.S3.Bucket.Name, record.S3.Object.Key, err)
		}
	}
	return nil
}

// processRecord runs the full pipeline for one uploaded object: download,
// convert to WebP, upload the result, then write its DynamoDB record.
func processRecord(ctx context.Context, record events.S3EventRecord) error {
	rawBucket := record.S3.Bucket.Name

	// S3 event keys are URL-encoded (e.g. a space becomes "+"), so they
	// must be decoded before being used as a real object key.
	rawKey, err := url.QueryUnescape(record.S3.Object.Key)
	if err != nil {
		return fmt.Errorf("decoding object key: %w", err)
	}

	// The CLI uploads originals as <uuid>.<ext>, so stripping the
	// extension recovers the UUID. It doubles as the processed object's
	// base name and the DynamoDB partition key.
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

	return saveMetadata(ctx, imagerecord.Record{
		ImageID:         imageID,
		RawBucket:       rawBucket,
		RawKey:          rawKey,
		ProcessedBucket: processedBucket,
		ProcessedKey:    processedKey,
		URL:             imagerecord.PublicURL(os.Getenv("AWS_REGION"), processedBucket, processedKey),
		Status:          "done",
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
	})
}

// downloadObject streams an S3 object straight to a local file at destPath.
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

// uploadObject uploads the local file at srcPath to bucket/key, tagged as
// image/webp since this is only ever called with the converted output.
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

// saveMetadata writes the final record for this image to DynamoDB. Using
// image_id as the key means a re-processed image simply overwrites its
// previous item rather than creating a duplicate.
func saveMetadata(ctx context.Context, rec imagerecord.Record) error {
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
