package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/IsraelSI11/climg/internal/imagerecord"
)

var uploadCmd = &cobra.Command{
	Use:   "upload <path>",
	Short: "Upload one image, or every image in a folder, to S3 for optimization",
	Long: `Upload one image, or every image in a folder, to S3 for optimization.

<path> can point to a single image file or to a directory (non-recursive);
every supported image directly inside the directory is uploaded concurrently.

Each upload gets a generated UUID key, and climg prints the final public
WebP URL immediately -- computed from that UUID, not by waiting for the
background Lambda to finish converting it. The URL becomes reachable a
few seconds later; use "climg status <image-id>" to confirm it's done.`,
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         runUpload,
}

func init() {
	rootCmd.AddCommand(uploadCmd)
}

// supportedImageExts are the input formats cwebp can decode; anything else
// found in a folder is silently skipped instead of erroring the batch.
var supportedImageExts = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".tif":  true,
	".tiff": true,
	".webp": true,
}

// uploadResult is what gets printed per file, in both text and --json mode.
type uploadResult struct {
	Path    string `json:"path"`
	ImageID string `json:"image_id,omitempty"`
	RawKey  string `json:"raw_key,omitempty"`
	URL     string `json:"url,omitempty"`
	Status  string `json:"status,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Caps how many uploads run at once so a big folder doesn't open hundreds
// of simultaneous connections to S3.
const maxConcurrentUploads = 8

// runUpload resolves the configured buckets/region, expands the given path
// into one or more image files, uploads them concurrently, and prints one
// result per file.
func runUpload(cmd *cobra.Command, args []string) error {
	target := args[0]

	rawBucket, _ := cmd.Flags().GetString("raw-bucket")
	processedBucket, _ := cmd.Flags().GetString("processed-bucket")
	if rawBucket == "" {
		return fmt.Errorf("no raw bucket configured: pass --raw-bucket or set CLIMG_RAW_BUCKET")
	}
	if processedBucket == "" {
		return fmt.Errorf("no processed bucket configured: pass --processed-bucket or set CLIMG_PROCESSED_BUCKET")
	}
	region, _ := cmd.Flags().GetString("region")
	asJSON, _ := cmd.Flags().GetBool("json")

	paths, err := collectImagePaths(target)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("no supported images found at %s", target)
	}

	ctx := context.Background()
	cfg, err := loadAWSConfig(ctx, region)
	if err != nil {
		return fmt.Errorf("loading AWS config: %w", err)
	}
	client := s3.NewFromConfig(cfg)

	// results[i] corresponds to paths[i]. Each goroutine only ever writes
	// its own index, so the slice is safely shared without a mutex.
	results := make([]uploadResult, len(paths))

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentUploads) // counting semaphore

	for i, path := range paths {
		wg.Add(1)
		go func(i int, path string) {
			defer wg.Done()
			sem <- struct{}{}        // blocks once 8 uploads are in flight
			defer func() { <-sem }() // frees a slot when this upload ends

			results[i] = uploadOne(ctx, client, rawBucket, processedBucket, region, path)
		}(i, path)
	}
	wg.Wait()

	failed := printUploadResults(results, asJSON)
	if failed > 0 {
		return fmt.Errorf("%d of %d uploads failed", failed, len(results))
	}
	return nil
}

// collectImagePaths returns target itself if it's a single image file, or
// every supported image directly inside it (no recursion) if it's a folder.
func collectImagePaths(target string) ([]string, error) {
	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		if !isSupportedImage(target) {
			return nil, fmt.Errorf("%s: unsupported image format", target)
		}
		return []string{target}, nil
	}

	entries, err := os.ReadDir(target) // returned already sorted by name
	if err != nil {
		return nil, err
	}

	var paths []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		full := filepath.Join(target, entry.Name())
		if isSupportedImage(full) {
			paths = append(paths, full)
		}
	}
	return paths, nil
}

// isSupportedImage reports whether path's extension is one cwebp can decode.
func isSupportedImage(path string) bool {
	return supportedImageExts[strings.ToLower(filepath.Ext(path))]
}

// uploadOne uploads a single file to the raw bucket under a fresh UUID key
// and returns its outcome as a value rather than an error, so one failed
// file doesn't abort the rest of a batch upload.
func uploadOne(ctx context.Context, client *s3.Client, rawBucket, processedBucket, region, path string) uploadResult {
	result := uploadResult{Path: path}

	f, err := os.Open(path)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer f.Close()

	imageID := uuid.NewString()
	ext := strings.ToLower(filepath.Ext(path))
	rawKey := imageID + ext

	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(rawBucket),
		Key:         aws.String(rawKey),
		Body:        f,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		result.Error = fmt.Errorf("uploading to s3: %w", err).Error()
		return result
	}

	result.ImageID = imageID
	result.RawKey = rawKey
	result.Status = "processing"
	result.URL = imagerecord.PublicURL(region, processedBucket, imageID+".webp")
	return result
}

// printUploadResults prints every result -- one JSON array in --json mode,
// or one tab-separated line per file otherwise -- and returns how many
// uploads failed, so the caller can decide the process's exit code.
func printUploadResults(results []uploadResult, asJSON bool) (failed int) {
	if asJSON {
		enc, _ := json.Marshal(results)
		fmt.Println(string(enc))
	}

	for _, r := range results {
		if r.Error != "" {
			failed++
			if !asJSON {
				fmt.Printf("%s\tERROR: %s\n", r.Path, r.Error)
			}
			continue
		}
		if !asJSON {
			fmt.Printf("%s\t%s\n", r.Path, r.URL)
		}
	}
	return failed
}
