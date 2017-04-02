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
)

type UploadCommand struct {
	Domain string `long:"domain" description:"Domain to be uploaded"`

	s3Filer *agent.S3Filer
	iamconn iamiface.IAMAPI
}

func (c *UploadCommand) init() {
	sess := session.New()
	s3b := bucket.New(s3.New(session.New()), Options.S3Bucket)
	c.s3Filer = agent.NewS3Filer(s3b, "")
	c.iamconn = iam.New(sess)
}

/*
aws iam upload-server-certificate \
	--server-certificate-name <value> \
    --certificate-body <value> \
    --private-key <value> \
    --certificate-chain <value>
*/
func (c *UploadCommand) Execute(args []string) error {
	c.init()

	privKey, err := c.get("privkey.pem")
	if err != nil {
		return err
	}
	cert, err := c.get("cert.pem")
	if err != nil {
		return err
	}
	certChain, err := c.get("chain.pem")
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	domain := strings.Replace(c.Domain, ".", "_", 0)

	name := fmt.Sprintf("%s_%s", domain, now.Format("200601021504"))

	req := &iam.UploadServerCertificateInput{
		CertificateBody:       &cert,
		CertificateChain:      &certChain,
		PrivateKey:            &privKey,
		ServerCertificateName: aws.String(name),
	}

	resp, err := c.iamconn.UploadServerCertificate(req)
	if err != nil {
		return err
	}

	certArn := aws.StringValue(resp.ServerCertificateMetadata.Arn)

	log.Printf("INFO: certificate has been uploaded: %s", certArn)

	return nil
}

func (c *UploadCommand) get(key string) (string, error) {
	fn := c.s3Filer.Join("aaa-data", Options.Email, "domain", c.Domain, key)
	blob, err := c.s3Filer.ReadFile(fn)
	if err != nil {
		log.Printf("ERROR: failed to read: %s", fn)
		return "", err
	}
	return string(blob), nil
}
