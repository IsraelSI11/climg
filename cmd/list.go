package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/spf13/cobra"

	"github.com/IsraelSI11/climg/internal/imagerecord"
)

var listCmd = &cobra.Command{
	Use:          "list",
	Short:        "List all images that have finished processing",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE:         runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
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
	out, err := client.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(table),
	})
	if err != nil {
		return fmt.Errorf("scanning dynamodb: %w", err)
	}

	var records []imagerecord.Record
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &records); err != nil {
		return fmt.Errorf("decoding dynamodb items: %w", err)
	}

	if asJSON {
		enc, err := json.Marshal(records)
		if err != nil {
			return err
		}
		fmt.Println(string(enc))
		return nil
	}

	if len(records) == 0 {
		fmt.Println("No hay imágenes procesadas todavía.")
		return nil
	}

	for _, rec := range records {
		fmt.Printf("%s\t%s\ts3://%s/%s\n", rec.ImageID, rec.Status, rec.ProcessedBucket, rec.ProcessedKey)
	}
	return nil
}
