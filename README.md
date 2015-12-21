# aaa

AAA is [ACME](https://tools.ietf.org/html/draft-ietf-acme-acme-01) Agent for AWS environment.
All information will be stored on [S3 with SSE-KMS](http://docs.aws.amazon.com/AmazonS3/latest/dev/UsingKMSEncryption.html).
This design allows us to run ACME agent in stateless.

## Current Status: alpha

- :heavy_check_mark: New Registration
- :heavy_check_mark: New Authorization
  - :heavy_check_mark: http-01
  - :heavy_check_mark: S3-based http-01
  - :heavy_check_mark: dns-01 with Route53 but [DNS01 is still broken on Let's Encrypt side](https://github.com/letsencrypt/boulder/pull/1295)
- :construction: Store data on S3 with SSE-KMS

## Usage

## Integrated libraries

- [github.com/aws/aws-sdk-go](https://github.com/aws/aws-sdk-go)
- [github.com/lestrrat/go-jwx](https://github.com/lestrrat/go-jwx)
- [github.com/spf13/afero](https://github.com/spf13/afero)
- [github.com/tent/http-link-go](https://github.com/tent/http-link-go)
- [gopkg.in/alecthomas/kingpin.v2](https://github.com/alecthomas/kingpin)

## Future work

- Integrate [S3 Event Notifications](http://docs.aws.amazon.com/AmazonS3/latest/dev/NotificationHowTo.html)..
  - To automate the installation of certificates (e.g. ELB)
  - To manage renewal of certificates (e.g. Use DynamoDB to manage certificates)
- Integrate Let's encrypt with Slack (e.g. `@bot let's encrypt with api.example.com` and the certificate will be available on S3...)
