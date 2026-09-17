package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/spf13/cobra"

	"github.com/IsraelSI11/climg/internal/imagerecord"
)

var statusCmd = &cobra.Command{
	Use:          "status <image-id>",
	Short:        "Check the processing status of an uploaded image",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

// runStatus looks up a single image_id in DynamoDB and prints what the
// Lambda has recorded so far, or a "still processing" message if it
// hasn't written the item yet.
func runStatus(cmd *cobra.Command, args []string) error {
	imageID := args[0]

	table, _ := cmd.Flags().GetString("table")
	if table == "" {
		return fmt.Errorf("no DynamoDB table configured: pass --table or set CLIMG_TABLE")
	}
	region, _ := cmd.Flags().GetString("region")
	asJSON, _ := cmd.Flags().GetBool("json")

	ctx := context.Background()
	cfg, err := loadAWSConfig(ctx, region)
	if err != nil {
		return fmt.Errorf("loading AWS config: %w", err)
	}

	client := dynamodb.NewFromConfig(cfg)
	out, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(table),
		// image_id is the table's partition key (see terraform/main.tf),
		// so this is the whole key needed to fetch a single item.
		Key: map[string]types.AttributeValue{
			"image_id": &types.AttributeValueMemberS{Value: imageID},
		},
	})
	if err != nil {
		return fmt.Errorf("querying dynamodb: %w", err)
	}

	// GetItem returns a nil Item (not an error) when the key doesn't
	// exist -- the expected case right after upload, before the Lambda
	// has had a chance to run.
	if out.Item == nil {
		if asJSON {
			fmt.Printf("{\"image_id\":%q,\"status\":\"processing\"}\n", imageID)
			return nil
		}
		fmt.Printf("%s todavía se está procesando (aún no aparece en DynamoDB).\n", imageID)
		return nil
	}

	var rec imagerecord.Record
	if err := attributevalue.UnmarshalMap(out.Item, &rec); err != nil {
		return fmt.Errorf("decoding dynamodb item: %w", err)
	}

	if asJSON {
		enc, err := json.Marshal(rec)
		if err != nil {
			return err
		}
		fmt.Println(string(enc))
		return nil
	}

	fmt.Printf("ID:       %s\n", rec.ImageID)
	fmt.Printf("Estado:   %s\n", rec.Status)
	fmt.Printf("Original: s3://%s/%s (privado)\n", rec.RawBucket, rec.RawKey)
	fmt.Printf("URL:      %s\n", rec.URL)
	fmt.Printf("Creado:   %s\n", rec.CreatedAt)
	return nil
}
