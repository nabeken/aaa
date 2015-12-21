package agent

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
)

// CreateCertificateRequest creates CSR in DER encoded in base64.
func CreateCertificateRequest(certPrivkey *rsa.PrivateKey, commonName string, domain ...string) (string, error) {
	dnsName := append([]string{commonName}, domain...)
	csr := &x509.CertificateRequest{
		SignatureAlgorithm: x509.SHA256WithRSA,
		PublicKeyAlgorithm: x509.RSA,
		Subject: pkix.Name{
			CommonName: commonName,
		},
		DNSNames: dnsName,
	}

	der, err := x509.CreateCertificateRequest(rand.Reader, csr, certPrivkey)
	if err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(der), nil
}
