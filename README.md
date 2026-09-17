<p align="center">
  <img src="assets/logo.png" alt="climg logo" width="480">
</p>

<p align="center">
  A CLI to upload images to S3 and get back optimized, web-ready WebP URLs — built for humans and AI agents alike.
</p>

## What is climg?

`climg` uploads images to an S3 bucket, where a Lambda function automatically converts them to WebP and republishes them on a second, public bucket. The CLI hands you back the final public URL immediately, so you can drop it straight into a website — no manual optimization step, no separate CDN setup.

```
climg upload photo.jpg
      │
      ▼
S3 (raw bucket)  ──(ObjectCreated event)──▶  Lambda (Go)
                                                │ decode → cwebp → re-encode
                                                ▼
                                        S3 (public bucket)  +  DynamoDB (metadata)
```

Everything runs serverless on AWS and is sized to stay within the [AWS Free Tier](https://aws.amazon.com/free/).

## Features

- **`upload`** — accepts a single image or a whole folder (uploaded concurrently), and prints the final public WebP URL instantly.
- **`status`** — checks whether a given image has finished processing.
- **`list`** — lists every image that has finished processing.
- **`--json`** on every command for scripting and AI agent consumption — clean, predictable output on stdout, errors on stderr.
- Infrastructure fully defined in Terraform: two S3 buckets, a DynamoDB table, an IAM role, and the Lambda + layer that does the conversion.

## Prerequisites

- [Go](https://go.dev/dl/) 1.27+
- [Terraform](https://developer.hashicorp.com/terraform/install) 1.5+
- [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html), configured with an IAM user that has permission to manage S3, DynamoDB, IAM, and Lambda (`aws configure`)
- An AWS account (the whole stack is designed to fit inside the free tier)

## Installation

Clone the repo and build the binary:

```bash
git clone https://github.com/IsraelSI11/climg.git
cd climg
go build -o climg .
```

Move `climg` somewhere on your `PATH` (e.g. `mv climg /usr/local/bin/`) to run it from anywhere.

## Deploying the infrastructure

The Lambda's deployment package and its `cwebp` layer are built by plain shell scripts, not by Terraform itself — run them once before the first `plan`/`apply`, and again any time you change the Lambda's Go code:

```bash
./lambda/layers/cwebp/build.sh        # downloads and packages the cwebp binary
./lambda/image-processor/build.sh     # compiles the Lambda and zips it
```

Then provision everything with Terraform:

```bash
cd terraform
terraform init
terraform plan
terraform apply
```

This creates: the raw and processed S3 buckets, the `climg-images` DynamoDB table, the Lambda function and its `cwebp` layer, the IAM role it runs as, and the S3 → Lambda event trigger.

Grab the resulting resource names from the outputs:

```bash
terraform output
```

## Configuration

Every command shares the same flags, each with an environment variable fallback so you don't have to repeat them on every call:

| Flag                 | Environment variable    | Description                                  |
| -------------------- | ------------------------ | --------------------------------------------- |
| `--raw-bucket`        | `CLIMG_RAW_BUCKET`       | S3 bucket where originals are uploaded        |
| `--processed-bucket`  | `CLIMG_PROCESSED_BUCKET` | Public S3 bucket serving optimized WebP images |
| `--table`             | `CLIMG_TABLE`            | DynamoDB table storing image metadata         |
| `--region`            | `CLIMG_REGION` / `AWS_REGION` | AWS region (default `us-east-1`)         |
| `--json`              | —                        | Print machine-readable JSON instead of text   |

The quickest way to set these up after `terraform apply`:

```bash
export CLIMG_RAW_BUCKET=$(terraform -chdir=terraform output -raw raw_bucket_name)
export CLIMG_PROCESSED_BUCKET=$(terraform -chdir=terraform output -raw processed_bucket_name)
export CLIMG_TABLE=$(terraform -chdir=terraform output -raw dynamodb_table_name)
```

## Usage

Upload a single image:

```bash
climg upload photo.jpg
# photo.jpg   https://climg-processed-....s3.us-east-1.amazonaws.com/<uuid>.webp
```

Upload every supported image in a folder (concurrently):

```bash
climg upload ./photos/
```

Check whether an image has finished processing:

```bash
climg status <image-id>
```

List every processed image:

```bash
climg list
```

Machine-readable output, for scripts or AI agents:

```bash
climg upload photo.jpg --json
# [{"path":"photo.jpg","image_id":"...","raw_key":"....jpg","url":"https://...webp","status":"processing"}]
```

The printed URL is computed from the upload's UUID and is correct immediately, but the object itself becomes reachable a few seconds later, once the Lambda finishes converting it — use `status` if you need to confirm it's actually live before using it.

## Project structure

```
.
├── main.go                    # CLI entry point
├── cmd/                       # Cobra commands (upload, status, list)
├── internal/imagerecord/      # DynamoDB item shape shared by the CLI and the Lambda
├── lambda/
│   ├── image-processor/       # Go Lambda: S3 → WebP → S3 → DynamoDB
│   └── layers/cwebp/          # Build script that packages the cwebp Lambda layer
└── terraform/                 # All AWS infrastructure (S3, DynamoDB, IAM, Lambda)
```

## Design notes

A few deliberate choices worth knowing about if you're reading the code:

- **Deterministic naming**: uploads are stored as `<uuid>.<ext>`, and the Lambda derives the DynamoDB key and the processed object's name (`<uuid>.webp`) directly from that. This is what lets the CLI print the final URL without waiting for processing to finish.
- **`cwebp` via a Lambda layer, not cgo or a container image**: the Lambda shells out to a statically-linked `cwebp` binary bundled as a layer, avoiding the complexity of cross-compiling cgo bindings or maintaining a container-image Lambda.
- **DynamoDB in `PROVISIONED` mode (5/5)**: this keeps the table inside DynamoDB's *Always Free* allowance (25 RCU/WCU), rather than on-demand billing, which is only free for an AWS account's first 12 months.

## License

MIT — see [LICENSE](LICENSE).
