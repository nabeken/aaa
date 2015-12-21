package agent

import (
	"crypto"
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/aws/aws-sdk-go/service/route53/route53iface"
)

const defaultRRSetTTL = 10

type ChallengeSolver interface {
	SolveChallenge(string) error
	CleanupChallenge(string) error
}

type DNSChallengeSolver struct {
	r53       *Route53Provider
	challenge Challenge
	domain    string
}

func NewDNSChallengeSolver(r53 *Route53Provider, challenge Challenge, domain string) *DNSChallengeSolver {
	return &DNSChallengeSolver{
		r53:       r53,
		challenge: challenge,
		domain:    domain,
	}
}

func (s *DNSChallengeSolver) SolveChallenge(keyAuthz string) error {
	keyAuthzDNSValue, err := valueForDNSChallenge(keyAuthz)
	if err != nil {
		return err
	}

	if err := s.r53.Update(s.domain, keyAuthzDNSValue); err != nil {
		return err
	}

	log.Printf("INFO: response for DNS Challenge has been deployed with %s", keyAuthzDNSValue)

	return nil
}

func (s *DNSChallengeSolver) CleanupChallenge(keyAuthz string) error {
	keyAuthzDNSValue, err := valueForDNSChallenge(keyAuthz)
	if err != nil {
		return err
	}
	return s.r53.Remove(s.domain, keyAuthzDNSValue)
}

type Challenge struct {
	URI              string     `json:"uri,omitempty"`
	Type             string     `json:"type"`
	Token            string     `json:"token"`
	KeyAuthorization string     `json:"keyAuthorization,omitempty"`
	Status           string     `json:"status,omitempty"`
	Error            *ACMEError `json:"error"`
}

type Route53Provider struct {
	r53api route53iface.Route53API
}

func NewRoute53Provider(r53api route53iface.Route53API) *Route53Provider {
	return &Route53Provider{
		r53api: r53api,
	}
}

func (p *Route53Provider) Remove(domain string, value string) error {
	hostedZone, err := p.findHostedZoneByDomain(domain)
	if err != nil {
		return err
	}

	change := &route53.Change{
		Action:            aws.String("DELETE"),
		ResourceRecordSet: rrsetForDNSChallenge(domain, value),
	}

	req := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: hostedZone.Id,
		ChangeBatch: &route53.ChangeBatch{
			Comment: aws.String("deleted by aaa"),
			Changes: []*route53.Change{change},
		},
	}

	resp, err := p.r53api.ChangeResourceRecordSets(req)
	if err != nil {
		return err
	}

	if aws.StringValue(resp.ChangeInfo.Status) == "INSYNC" {
		return nil
	}

	return p.waitUntilInSync(domain, resp.ChangeInfo.Id)
}

func (p *Route53Provider) Update(domain string, value string) error {
	hostedZone, err := p.findHostedZoneByDomain(domain)
	if err != nil {
		return err
	}

	change := &route53.Change{
		Action:            aws.String("UPSERT"),
		ResourceRecordSet: rrsetForDNSChallenge(domain, value),
	}

	req := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: hostedZone.Id,
		ChangeBatch: &route53.ChangeBatch{
			Changes: []*route53.Change{change},
			Comment: aws.String("updated by aaa"),
		},
	}

	resp, err := p.r53api.ChangeResourceRecordSets(req)
	if err != nil {
		return err
	}

	if aws.StringValue(resp.ChangeInfo.Status) == "INSYNC" {
		return nil
	}

	return p.waitUntilInSync(domain, resp.ChangeInfo.Id)
}

func (p *Route53Provider) waitUntilInSync(domain string, changeId *string) error {
	req := &route53.GetChangeInput{
		Id: changeId,
	}

	// http://docs.aws.amazon.com/Route53/latest/APIReference/API_ChangeResourceRecordSets.html
	// > In rare circumstances, propagation can take up to 30 minutes
	Debug("waiting for RRSets change is INSYNC")
	last := time.Duration(30 * time.Minute)
	for begin := time.Now(); time.Since(begin) < last; time.Sleep(5 * time.Second) {
		resp, err := p.r53api.GetChange(req)
		if err != nil {
			return err
		}

		status := aws.StringValue(resp.ChangeInfo.Status)
		Debug("ChangeInfo.Status is " + status)

		if status == "INSYNC" {
			Debug("RRSets change has been synced")
			return nil
		}
	}

	return fmt.Errorf("aaa: RRSets change for %s is still not INSYNC for %s", domain, last)
}

func (p *Route53Provider) findHostedZoneByDomain(domain string) (*route53.HostedZone, error) {
	// 1. Add `.` to the last if domain does not have it
	if domain[len(domain)-1] != '.' {
		domain += "."
	}

	var zones []*route53.HostedZone

	// 2. retrieve all hosted zone until IsTruncated == false OR 1000 attempts are made
	var nextMarker *string
	for i := 0; i < 1000; i++ {
		resp, err := p.r53api.ListHostedZones(&route53.ListHostedZonesInput{
			Marker: nextMarker,
		})
		if err != nil {
			return nil, err
		}
		zones = append(zones, resp.HostedZones...)

		if !*resp.IsTruncated {
			break
		}
		nextMarker = resp.NextMarker
	}

	// 3. find hosted zones that has the same suffix
	var zone *route53.HostedZone
	for _, z := range zones {
		if strings.HasSuffix(domain, aws.StringValue(z.Name)) {
			if zone == nil {
				zone = z
				continue
			}

			// If new hosted zone name is the same length or larger than current zone,
			// swap this.
			if len(aws.StringValue(z.Name)) >= len(aws.StringValue(zone.Name)) {
				zone = z
			}
		}
	}

	if zone == nil {
		return nil, fmt.Errorf("aaa: no hosted zone found for %s", domain)
	}

	Debug(domain)
	Debug(aws.StringValue(zone.Name))

	return zone, nil
}

func validatingLabel(domain string) string {
	return "_acme-challenge." + domain
}

func rrsetForDNSChallenge(domain, value string) *route53.ResourceRecordSet {
	return &route53.ResourceRecordSet{
		Name: aws.String(validatingLabel(domain)),
		TTL:  aws.Int64(defaultRRSetTTL),
		Type: aws.String("TXT"),
		ResourceRecords: []*route53.ResourceRecord{
			&route53.ResourceRecord{
				Value: aws.String(`"` + value + `"`),
			},
		},
	}
}

// FindDNSChallenge finds DNS-01 Challenge from challenge and its combinations.
// If it does not find the challenge, it will return (Challenge{}, false).
func FindDNSChallenge(resp *Authorization) (Challenge, bool) {
	return findChallenge(resp, "dns-01")
}

// FindHTTPChallenge finds HTTP-01 Challenge from challenge and its combinations.
// If it does not find the challenge, it will return (Challenge{}, false).
func FindHTTPChallenge(resp *Authorization) (Challenge, bool) {
	return findChallenge(resp, "http-01")
}

func findChallenge(resp *Authorization, ctype string) (Challenge, bool) {
	for i, c := range resp.Challenges {
		if c.Type == ctype {
			for _, combi := range resp.Combinations {
				// FIXME: assume that combination has only 1 element so far.
				if len(combi) != 1 {
					return Challenge{}, false
				}
				for _, challengeIndex := range combi {
					if i == challengeIndex {
						return c, true
					}
				}
			}
		}
	}
	return Challenge{}, false
}

// https://tools.ietf.org/html/draft-ietf-acme-acme-01#section-7.5
func valueForDNSChallenge(keyAuthz string) (string, error) {
	hasher := crypto.SHA256.New()
	fmt.Fprint(hasher, keyAuthz)
	sum := hasher.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(sum), nil
}
