package command

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/nabeken/aaa/agent"
	"github.com/nabeken/aws-go-s3/bucket"
	"github.com/pkg/errors"
)

type UploadService struct {
	Domain string
	Email  string

	S3Filer *agent.S3Filer
	IAMconn iamiface.IAMAPI
}

/*
aws iam upload-server-certificate \
	--server-certificate-name <value> \
    --certificate-body <value> \
    --private-key <value> \
    --certificate-chain <value>
*/

func (svc *UploadService) get(key string) (string, error) {
	fn := svc.S3Filer.Join("aaa-data", svc.Email, "domain", svc.Domain, key)
	blob, err := svc.S3Filer.ReadFile(fn)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read '%s'", fn)
	}
	return string(blob), nil
}

func (svc *UploadService) buildUploadInput() (*iam.UploadServerCertificateInput, error) {
	privKey, err := svc.get("privkey.pem")
	if err != nil {
		return nil, err
	}
	cert, err := svc.get("cert.pem")
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	domain := strings.Replace(svc.Domain, ".", "_", 0)
	name := fmt.Sprintf("%s_%s", domain, now.Format("200601021504"))
	return &iam.UploadServerCertificateInput{
		CertificateBody:       &cert,
		PrivateKey:            &privKey,
		ServerCertificateName: aws.String(name),
	}, nil
}

func (svc *UploadService) Run() (string, error) {
	req, err := svc.buildUploadInput()
	if err != nil {
		return "", errors.Wrap(err, "failed to build a upload request")
	}

	resp, err := svc.IAMconn.UploadServerCertificate(req)
	if err != nil {
		return "", errors.Wrap(err, "failed to upload to IAM")
	}

	return aws.StringValue(resp.ServerCertificateMetadata.Arn), nil
}

type UploadCommand struct {
	Domain string `long:"domain" description:"Domain to be uploaded"`
}

func (c *UploadCommand) Execute(args []string) error {
	sess := NewAWSSession()
	s3b := bucket.New(s3.New(sess), Options.S3Bucket)
	arn, err := (&UploadService{
		Domain:  c.Domain,
		Email:   Options.Email,
		S3Filer: agent.NewS3Filer(s3b, ""),
		IAMconn: iam.New(sess),
	}).Run()
	if err != nil {
		return err
	}

	log.Printf("INFO: certificate has been uploaded: %s", arn)
	return nil
}
