# aaa

AAA is an [ACME](https://ietf-wg-acme.github.io/acme/) Agent for AWS environment.
All information is stored on [S3 with SSE-KMS](http://docs.aws.amazon.com/AmazonS3/latest/dev/UsingKMSEncryption.html).
This design allows us to run ACME agent in stateless (e.g. AWS Lambda).

## Current Status: beta

AAA works well with DNS-01 on LE's production environment. It means you can issue certificates without HTTP servers.

- :heavy_check_mark: New Registration
- :heavy_check_mark: New Authorization
  - :heavy_check_mark: dns-01 with Route53
- :heavy_check_mark: Create CSR with [SAN (Subject Alternative Name)](https://en.wikipedia.org/wiki/SubjectAltName)
- :heavy_check_mark: Issue certificates
- :heavy_check_mark: Store data on S3 with SSE-KMS
- :heavy_check_mark: Renewal management by utilizing S3
- :heavy_check_mark: AWS Lambda build
  - :heavy_check_mark: authz, cert
  - :heavy_check_mark: automatic certificates renewal
- :heavy_check_mark: Confirmed that it works well with ELB

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
export AAA_DIRECTORY_URL=https://acme-v01.api.letsencrypt.org/directory
```

## Registration

```sh
aaa reg --email you@example.com --s3-bucket YourBucket --s3-kms-key xxxx
Please agree with TOS found at https://letsencrypt.org/documents/LE-SA-v1.0.1-July-27-2015.pdf
```

`aaa` prints the message that you must agree TOS to proceed. After you read it and agree with it:

```sh
aaa reg --email you@example.com --s3-bucket YourBucket --s3-kms-key xxxx --agree https://letsencrypt.org/documents/LE-SA-v1.0.1-July-27-2015.pdf
```

## Authorization

`aaa` implements DNS-01 solver. You must provide Route53 environment.

```sh
aaa authz --email you@example.com --s3-bucket YourBucket --s3-kms-key xxxx --domain le-test-01.example.com --challenge dns-01
```

Bonus: You authorize more domains, you will get a certificate that has SAN for your domains.

```sh
aaa authz --email you@example.com --s3-bucket YourBucket --s3-kms-key xxxx --domain le-test-02.example.com --challenge dns-01
```

## Certificate issuance

Let's issue a certifiate for two domains `le-test-0[12].example.com`. If you don't want to issue a certificate with SAN, just drop `--domain` argument.

```
aaa cert --email you@example.com --s3-bucket YourBucket --s3-kms-key xxxx --cn le-test-01.example.com --domain le-test-02.example.com
```

## Listing all information

To show all accounts and certificates, you can use `ls` subcommand like this:

```sh
aaa ls --s3-bucket YourBucket --s3-kms-key xxxx | jq -r .
[
  {
    "email": "letest-stag@example.com",
    "domain": "le-test-dns-01.example.com",
    "authorization": {
      "expires": "2016-11-06T16:57:22Z"
    },
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

Please note that information is encoded in JSON. This information will be used for certificate renewal management and
it allows another processes to consume the info easily.

## Certificate renewal

If the authorization is still available, you can just issue the certificate again by passing `--renewal` flag.
Otherwise, you need to begin with authorization by using `authz` subcommand. See the above.`Authorization` section.

```sh
aaa cert --email you@example.com --s3-bucket YourBucket --s3-kms-key xxxx --renewal --cn le-test-01.example.com --domain le-test-02.example.com
```

If you want to regenerate the key, add `--renewal-key` flag too:

```sh
aaa cert --email you@example.com --s3-bucket YourBucket --s3-kms-key xxxx --renewal --renewal-key --cn le-test-01.example.com --domain le-test-02.example.com
```

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

We integrate `aaa` with Slack's [Slash Commands](https://api.slack.com/slash-commands). To do this, we need:

- AWS API Gateway that invokes...
- AWS Lambda Function `aaa-dispatcher` synchronously that invokes ...
- AWS Lambda Function `aaa-executor` asynchronously that invokes `aaa` command-line application

We should respond to Slash Commands within 3 seconds so we invoke `aaa-executor` asynchronously.

Steps: (assumes that you have already the KMS key and the S3 bucket)

1. Create a Slash Command `/letsencrypt` in Slack but let the endpoint URL empty
2. Create a configuration `aaa_lambda.toml`
3. Build `lambda.zip` to bundle two Lambda Functions
4. Create two Lambda Functions `aaa-dispatcher` and `aaa-executor`
5. Create a API Gateway `aaa-dispatcher-gateway`
6. Update the Slash Command to fill the endpoint URL that points to the `aaa-dispatcher-gateway`

Here we go!

### Create a Slash Command

Please see https://api.slack.com/slash-commands for detail. Here, `token` parameter is needed.

### Create a configuration

```sh
# first, copy sample
cp aaa_lambda.toml.samlple aaa_lambda.toml
```

`encrypted_slack_token` in `dispatcher` section can be generated by awscli:

```sh
# <TOKEN> is a token for the slash command we created
aws kms encrypt --key-id "<KMS_KEY_ID>" --plaintext "<TOKEN>" | jq -r .CiphertextBlob
```

Then, please adjust `s3_bucket`, `kms_key` in `executor` section.

In default, `directory_url` points to the staging.
If you want to use the production, you should change the url to `https//acme-v01.api.letsencrypt.org/directory`.

### Build `lambda.zip` to bundle Lambda Function

```sh
make
```

:)

### Create two Lambda Functions

We need a IAM role for Lambda environment.

Please create a IAM role `aaa-lambda`.
It has the following managed policies:

- `AmazonS3FullAccess`
- `AmazonRoute53FullAccess`
- `CloudWatchLogsFullAccesss`
- `AWSLambdaRole`
- `IAMFullAccess`

and it has the following inline policy `aaa-kms`:

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

For `aaa-dispatcher`:

```sh
aws lambda create-function \
  --function-name aaa-dispatcher \
  --description 'AAA Dispatcher' \
  --runtime nodejs \
  --role <IAM_ROLE_ARN> \
  --handler aaa_dispatcher.handler \
  --timeout 10 \
  --zip-file fileb://lambda.zip
```

For `aaa-executor`:

```sh
aws lambda create-function \
  --function-name aaa-executor \
  --description 'AAA Executor' \
  --runtime nodejs \
  --role <IAM_ROLE_ARN> \
  --handler aaa_executor.handler \
  --timeout 300 \
  --zip-file fileb://lambda.zip
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

We do automatic certificates renewal by Lambda Function `aaa-schedular` with scheduled events.

Create a Lambda Function `aaa-schedular`:

```sh
aws lambda create-function \
  --function-name aaa-schedular \
  --description 'AAA Schedular for automatic certificate renewal' \
  --runtime nodejs \
  --role <IAM_ROLE_ARN> \
  --handler aaa_schedular.handler \
  --timeout 300 \
  --zip-file fileb://lambda.zip
```

Currently, you can setup Scheduled Events only from AWS Management Console.
Please add a scheduled event source to `aaa-Schedular`. 1 day is sufficient.

## Integrated libraries

- [github.com/aws/aws-sdk-go](https://github.com/aws/aws-sdk-go)
- [github.com/lestrrat/go-jwx](https://github.com/lestrrat/go-jwx)
- [github.com/tent/http-link-go](https://github.com/tent/http-link-go)
- [gopkg.in/alecthomas/kingpin.v2](https://github.com/alecthomas/kingpin)

## Future work

- Integrate [S3 Event Notifications](http://docs.aws.amazon.com/AmazonS3/latest/dev/NotificationHowTo.html) ...
  - To automate the installation of certificates (e.g. ELB)
  - To manage renewal of certificates (e.g. Use DynamoDB as a database)
