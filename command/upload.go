package command

import (
	"crypto/x509"
	"encoding/pem"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/acm"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/go-acme/lego/v3/certcrypto"
	"github.com/nabeken/aaa/agent"
	"github.com/nabeken/aws-go-s3/bucket"
	"github.com/pkg/errors"
)

type UploadService struct {
	Domain string
	Email  string

	S3Filer *agent.S3Filer
	ACMconn *acm.ACM
}

/*
aws acm import-certificate \
	--certificate file://Certificate.pem \
	--certificate-chain file://CertificateChain.pem \
    --private-key file://PrivateKey.pem
*/

func (svc *UploadService) get(key string) ([]byte, error) {
	fn := svc.S3Filer.Join("aaa-data", "v2", svc.Email, "domain", svc.Domain, key)
	blob, err := svc.S3Filer.ReadFile(fn)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read '%s'", fn)
	}
	return blob, nil
}

func (svc *UploadService) buildImportCertificateInput() (*acm.ImportCertificateInput, error) {
	privKey, err := svc.get("privkey.pem")
	if err != nil {
		return nil, err
	}

	cert, err := svc.get("cert.pem")
	if err != nil {
		return nil, err
	}

	certs, err := certcrypto.ParsePEMBundle(cert)
	if err != nil {
		return nil, err
	}

	return &acm.ImportCertificateInput{
		Certificate:      svc.pemEncode(certs[0]),
		PrivateKey:       privKey,
		CertificateChain: svc.pemEncode(certs[1]),
	}, nil
}

func (svc *UploadService) pemEncode(cert *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
}

func (svc *UploadService) Run() (string, error) {
	req, err := svc.buildImportCertificateInput()
	if err != nil {
		return "", errors.Wrap(err, "failed to build an import request")
	}

	resp, err := svc.ACMconn.ImportCertificate(req)
	if err != nil {
		return "", errors.Wrap(err, "failed to import into ACM")
	}

	return aws.StringValue(resp.CertificateArn), nil
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
		ACMconn: acm.New(sess),
	}).Run()
	if err != nil {
		return err
	}

	log.Printf("INFO: certificate has been uploaded: %s", arn)
	return nil
}
