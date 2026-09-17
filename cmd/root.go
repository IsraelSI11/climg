/*
Copyright © 2026 Israel
*/
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "climg",
	Short: "Upload and Manage optimized images in S3",
	Long: `Upload and Manage optimized images in S3. 
	
	This CLI is meant to be used for humans and agents, the main purpose of this tool
	is to enhance the security and performance of the management of webapps assets.
	`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().String("region", envOrDefault("CLIMG_REGION", envOrDefault("AWS_REGION", "us-east-1")), "AWS region")
	rootCmd.PersistentFlags().String("raw-bucket", os.Getenv("CLIMG_RAW_BUCKET"), "S3 bucket where originals are uploaded")
	rootCmd.PersistentFlags().String("processed-bucket", os.Getenv("CLIMG_PROCESSED_BUCKET"), "S3 bucket serving optimized WebP images")
	rootCmd.PersistentFlags().String("table", os.Getenv("CLIMG_TABLE"), "DynamoDB table storing image metadata")
	rootCmd.PersistentFlags().Bool("json", false, "Print machine-readable JSON instead of human-readable text")
}

// envOrDefault lets every flag above fall back to an environment variable
// before its hardcoded default, so climg can run unattended (CI, agents)
// without repeating --region/--raw-bucket/etc. on every invocation.
func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
