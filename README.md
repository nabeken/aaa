# aaa

AAA is an yet another integration with [go-acme/lego](https://github.com/go-acme/lego) for AWS environment. All information is persisted on [S3 with SSE-KMS](http://docs.aws.amazon.com/AmazonS3/latest/dev/UsingKMSEncryption.html) so that it can run with serverless.

## News as of March 16, 2025

I've tagged the codebase at of March 16, 2025 as `v2.0.0`. `master` branch is for for v3 with upcoming major refactoring.

## Integrations

- Authorize domains with Route53
- Persist the registration information and certificate on S3
- Upload certificates to ACM
- Run with Slack command over the API Gateway + Lambda (deploy with [Apex](https://apex.run/))

## Installation

aaa is still in beta so no pre-built binaries are available.

```sh
go get -u github.com/nabeken/aaa
```

## S3 Bucket and KMS Usage

`AAA` requires:

- AWS KMS Encryption Key for encrypting all data in the S3 bucket and token in the Lambda Function
- A dedicated S3 bucket that holds
  - `/aaa-data/*`: will be used for all generated secrets and certificates so under this prefix MUST be encrypted

To protect you from uploading data without encryption, I highly recommend you to add a bucket policy like this:

```json
{
  "Version": "2012-10-17",
  "Id": "PutObjPolicy",
  "Statement": [
    {
      "Sid": "DenyUnEncryptedObjectUploads",
      "Effect": "Deny",
      "Principal": "*",
      "Action": "s3:PutObject",
      "Resource": "arn:aws:s3:::YourBucket/aaa-data/*",
      "Condition": {
        "StringNotEquals": {
          "s3:x-amz-server-side-encryption": "aws:kms"
        }
      }
    }
  ]
}
```

## Usage

To issue the certificate, you must:

- register your account key pair
- authorize your domain with solving the challenge

Finally, you are able to request ACME server to issue your certificates.

In default, ACME API endpoint in `aaa` points to LE's staging environment.
After you grasp how `aaa` works, you can point the endpoint to LE's production environment.

```sh
export AAA_DIRECTORY_URL=https://acme-v02.api.letsencrypt.org/directory
```

## Registration

```sh
aaa reg --email you@example.com --s3-bucket YourBucket --s3-kms-key xxxx
```

`aaa` prints the message that you must agree TOS to proceed. You can agree with `--agree-tos`.

## Certificate issuance

Let's issue a certifiate for two domains `le-test-0[12].example.com`. If you don't want to issue a certificate with SAN, just drop `--domain` argument.

```
aaa cert \
  --email you@example.com \
  --s3-bucket YourBucket \
  --s3-kms-key xxxx \
  --cn le-test-01.example.com \
  --domain le-test-02.example.com
```

You can use this command to renew the cert. `aaa` will reuse the existing private key, or add `--create-key` for renew the key.

## Uploading certificate to ACM

```
aaa upload \
  --email you@example.com \
  --s3-bucket YourBucket \
  --domain le-test-02.example.com
```

## Listing all information

To show all accounts and certificates, you can use `ls` subcommand like this:

```sh
aaa ls --s3-bucket YourBucket --s3-kms-key xxxx | jq -r .
[
  {
    "email": "letest-stag@example.com",
    "domain": "le-test-dns-01.example.com",
    "certificate": {
      "not_before": "2016-01-11T16:02:00Z",
      "not_after": "2016-04-10T16:02:00Z",
      "san": [
        "le-test-dns-01.example.com"
      ]
    }
  }
]
```

Please note that information is encoded in JSON. This information will be used for certificate renewal management and it allows another processes to consume the info easily.

## Certificate distribution

Create an R/O IAM role/user for a specific prefix like `/aaa-data/foobar@example.com/domain/le-test.example.com` like this:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "Stmt1453475376000",
            "Effect": "Allow",
            "Action": [
                "s3:GetObject",
                "s3:PutObject"
            ],
            "Resource": [
                "arn:aws:s3:::example-bucket/aaa-data/foobar@example.com/domain/le-test.example.com/*"
            ]
        },
        {
            "Sid": "Stmt1453475425000",
            "Effect": "Allow",
            "Action": [
                "s3:ListBucket"
            ],
            "Resource": [
                "arn:aws:s3:::example-bucket"
            ]
        }
    ]
}
```

And add this user to KMS users.

Do `aaa sync` with the prefix periodically:

```sh
aaa sync --email foobar@example.com --domain le-test.example.com --s3-bucket example-bucket
```

Profit!

Or

1. Create an R/O IAM role/user for a specific prefix like `/aaa-data/foobar@example.com/domain/le-test.example.com`
2. When automatic renewal process puts new certificate on S3, S3 notifications will be generated
3. Respond with the notification with Lambda Function and update the certificate
4. Profit!

## Slack integration with AWS Lambda

We integrate `aaa` with Slack's [Slash Commands](https://api.slack.com/slash-commands) by Apex. To do this, we need:

- AWS API Gateway that invokes...
- AWS Lambda Function `aaa_dispatcher` synchronously that invokes ...
- AWS Lambda Function `aaa_executor` asynchronously

We should respond to Slash Commands within 3 seconds so we have to defer to `aaa_executor` asynchronously.

Steps: (assumes that you have already the KMS key and the S3 bucket)

1. Create a Slash Command `/letsencrypt` in Slack but let the endpoint URL be empty
2. Create a IAM role for the lambda functions
3. Create a Apex configuration `project.json`
5. Deploy the lambda functions by apex
4. Create a API Gateway `aaa-dispatcher-gateway` and deploy it
6. Update the Slash Command to fill the endpoint URL that points to the `aaa-dispatcher-gateway`

Here we go!

### Create a Slash Command

Please see https://api.slack.com/slash-commands for detail. Here, `token` parameter is needed.

### Create a IAM role

We need a IAM role for Lambda environment.

Please create a IAM role `aaa-lambda`. It should have the following managed policies:

- `AmazonS3FullAccess`
- `AmazonRoute53FullAccess`
- `CloudWatchLogsFullAccesss`
- `AWSLambdaRole`
- `IAMFullAccess`

and the following inline policy `aaa-kms`:

```json
{
  "Version": "2012-10-17",
    "Statement": [
      {
        "Effect": "Allow",
        "Action": [
          "kms:Decrypt"
        ],
        "Resource": [
          "<KMS_KEY_ARN>"
        ]
      }
    ]
}
```

### Create a Apex configuration

```sh
# Copy sample
cp project.json.example project.json

# Fill it
$EDITOR project.json
```

### Deploy the functions

```sh
# dryrun
apex deploy -D

# If everything is okay, then shipit
apex deploy
```

### Create a API Gateway

Create an API named `aaa-dispatcher-gateway`.

We really don't care about how resources are located but we need a resource that accepts `POST` method and some configurations for the resource:

- Integration Request
  - ***Integration type***: Lambda Function
  - ***Lambda Region***: (Select your region)
  - ***Lambda Function***: `aaa-dispatcher`
  - ***Mapping Templates***: Set `Content-Type: application/x-www-form-urlencoded` and create mapping tables that converts POST form request into JSON. You can find some examples on the Internet.
    - http://qiita.com/satetsu888/items/40fc387735192b794da8 (in Japanese)
    - https://forums.aws.amazon.com/thread.jspa?messageID=673012&tstart=0#673012

Then, deploy API. You will get a invoke URL for the resource.

### Update the Slash Command to fill the endpoint URL

Finally, you can fill the URL in `Integration Settings`.

## Automatic renewal

You can invoke `aaa_scheduler` lambda function by CloudWatch Events. The scheduler will invoke the executor lambda function when a domain requires authz or cert renewal 30 days before it expires.
