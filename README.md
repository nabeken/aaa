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
- :heavy_check_mark: Create CSR with [SAN (Subject Alternative Name)](https://en.wikipedia.org/wiki/SubjectAltName)
- :heavy_check_mark: Issue certificates
- :construction: Store data on S3 with SSE-KMS

## Installation

aaa is still in alpha so no pre-built binaries are available.

```sh
go get -u github.com/nabeken/aaa
```

## Usage

To issue the certificate, you must:

- register your account key pair
- authorize your domain with solving the challenge

Finally, you are able to request ACME server to issue your certificates.

## Registration

```sh
aaa reg --email you@example.com
Please agree with TOS found at https://letsencrypt.org/documents/LE-SA-v1.0.1-July-27-2015.pdf
```

`aaa` prints the message that you must agree TOS to proceed. After you read it and agree with it:

```sh
aaa reg --email you@example.com --agree https://letsencrypt.org/documents/LE-SA-v1.0.1-July-27-2015.pdf
```

## Authorization

`aaa` implements 3 solver for challenges: `http-01`, `s3-http-01` and `dns-01`.

- http-01: This is for debugging. Do not use unless you know what this is.
- s3-http-01: This is a workaround untill `dns-01` is properly landed on Let's Encrypt's side.
- dns-01: This will be our main method to automate things but it does not work due to LE's bad.

We introduce `s3-http-01` method here.

With `s3-http-01`, information needed for challenge is stored on S3 bucket by `aaa`.
A target web server must be configured in advance to redirect a request from LE to S3 bucket.

```txt
+-----+   new-authz    +-------------+   GET /.well-known/acme-challenge/{token}  +--------+
|     |  ----------->  |             | -----------------------------------------> |        |
| aaa |   challenge    | ACME server |  redirect to s3://foobar/.well-known/....  | target |
|     |  <-----------  |             | <----------------------------------------- |        |
+--+--+                +-------------+                                            +--------+
   |                         |
   |                         | GET /.well-known/acme-challenge/{token}
   |                        \|/
   |         put        +----------+
   +------------------> |    S3    |
                        +----------+
```

Example for nginx:

```txt
location /.well-known/acme-challenge/ {
    return 302 https://s3-ap-northeast-1.amazonaws.com/your-s3-bucket$request_uri;
}
```

To communicate with S3, you also need to setup AWS credentials. `aaa` currently utilizes the default behavior of `aws-sdk-go`.
Please see [Configuring SDK](https://github.com/aws/aws-sdk-go/wiki/configuring-sdk) for detail.

It's time to authorize!

```sh
aaa authz --email you@example.com --domain le-test-01.example.com --challenge s3-http-01 --s3-bucket your-s3-bucket
```

Bounus: You authorize more domain, you will get a certificate that has SAN for your domains.

```sh
aaa authz --email you@example.com --domain le-test-02.example.com --challenge s3-http-01 --s3-bucket your-s3-bucket
```

## certificate issuance

Let's issue a certifiate for two domains `le-test-0[12].example.com`. If you don't want to issue a certificate with SAN, just drop `--domain` argument.

```
aaa cert --email you@example.com --common-name le-test-01.example.com --domain le-test-02.example.com
```

## Integrated libraries

- [github.com/aws/aws-sdk-go](https://github.com/aws/aws-sdk-go)
- [github.com/lestrrat/go-jwx](https://github.com/lestrrat/go-jwx)
- [github.com/spf13/afero](https://github.com/spf13/afero)
- [github.com/tent/http-link-go](https://github.com/tent/http-link-go)
- [gopkg.in/alecthomas/kingpin.v2](https://github.com/alecthomas/kingpin)

## Future work

- Integrate [S3 Event Notifications](http://docs.aws.amazon.com/AmazonS3/latest/dev/NotificationHowTo.html) ...
  - To automate the installation of certificates (e.g. ELB)
  - To manage renewal of certificates (e.g. Use DynamoDB as database)
- Integrate Let's encrypt with Slack (e.g. `@bot let's encrypt with api.example.com` and the certificate will be available on S3...)