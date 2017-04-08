package command

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/nabeken/aaa/agent"
	"github.com/nabeken/aws-go-s3/bucket"
	"github.com/pkg/errors"
)

type UploadService struct {
	Domain   string
	S3Bucket string
	Email    string

	s3Filer *agent.S3Filer
	iamconn iamiface.IAMAPI
}

/*
aws iam upload-server-certificate \
	--server-certificate-name <value> \
    --certificate-body <value> \
    --private-key <value> \
    --certificate-chain <value>
*/

func (svc *UploadService) init() {
	sess := session.New()
	s3b := bucket.New(s3.New(session.New()), svc.S3Bucket)
	svc.s3Filer = agent.NewS3Filer(s3b, "")
	svc.iamconn = iam.New(sess)
}

func (svc *UploadService) get(key string) (string, error) {
	fn := svc.s3Filer.Join("aaa-data", svc.Email, "domain", svc.Domain, key)
	blob, err := svc.s3Filer.ReadFile(fn)
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
	certChain, err := svc.get("chain.pem")
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	domain := strings.Replace(svc.Domain, ".", "_", 0)
	name := fmt.Sprintf("%s_%s", domain, now.Format("200601021504"))
	return &iam.UploadServerCertificateInput{
		CertificateBody:       &cert,
		CertificateChain:      &certChain,
		PrivateKey:            &privKey,
		ServerCertificateName: aws.String(name),
	}, nil
}

func (svc *UploadService) Run() (string, error) {
	svc.init()

	req, err := svc.buildUploadInput()
	if err != nil {
		return "", errors.Wrap(err, "failed to build a upload request")
	}

	resp, err := svc.iamconn.UploadServerCertificate(req)
	if err != nil {
		return "", errors.Wrap(err, "failed to upload to IAM")
	}

	return aws.StringValue(resp.ServerCertificateMetadata.Arn), nil
}

type UploadCommand struct {
	Domain string `long:"domain" description:"Domain to be uploaded"`
}

func (c *UploadCommand) Execute(args []string) error {
	arn, err := (&UploadService{
		Domain:   c.Domain,
		S3Bucket: Options.S3Bucket,
		Email:    Options.Email,
	}).Run()
	if err != nil {
		return err
	}

	log.Printf("INFO: certificate has been uploaded: %s", arn)
	return nil
}
