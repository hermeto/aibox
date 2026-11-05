// Package awsapi implements the read-only AWS calls used by `aibox doctor` with the AWS
// SDK for Go v2. Every call here is a Describe/List/Get operation; nothing mutates.
package awsapi

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/servicequotas"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/hermeto/aibox/internal/doctor"
)

// Client is the SDK-backed doctor.AWSClient.
type Client struct {
	ec2 *ec2.Client
	sq  *servicequotas.Client
	sts *sts.Client
}

// New is a doctor.AWSFactory: it loads the default credential chain and returns
// doctor.ErrNoCredentials (wrapped) when nothing resolves, without calling AWS APIs.
func New(ctx context.Context, region string) (doctor.AWSClient, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	rctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if _, err := cfg.Credentials.Retrieve(rctx); err != nil {
		return nil, fmt.Errorf("%w: %v", doctor.ErrNoCredentials, err) //nolint:errorlint // chain error is informational
	}
	return &Client{ec2: ec2.NewFromConfig(cfg), sq: servicequotas.NewFromConfig(cfg), sts: sts.NewFromConfig(cfg)}, nil
}

// CallerIdentity calls sts:GetCallerIdentity.
func (c *Client) CallerIdentity(ctx context.Context) (string, string, error) {
	out, err := c.sts.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", "", err
	}
	return aws.ToString(out.Account), aws.ToString(out.Arn), nil
}

// InstanceTypesOffered calls ec2:DescribeInstanceTypeOfferings (location type region).
func (c *Client) InstanceTypesOffered(ctx context.Context, types []string) (map[string]bool, error) {
	offered := map[string]bool{}
	p := ec2.NewDescribeInstanceTypeOfferingsPaginator(c.ec2, &ec2.DescribeInstanceTypeOfferingsInput{
		LocationType: ec2types.LocationTypeRegion,
		Filters:      []ec2types.Filter{{Name: aws.String("instance-type"), Values: types}},
	})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, o := range page.InstanceTypeOfferings {
			offered[string(o.InstanceType)] = true
		}
	}
	return offered, nil
}

// InstanceVCPUs calls ec2:DescribeInstanceTypes.
func (c *Client) InstanceVCPUs(ctx context.Context, types []string) (map[string]int, error) {
	its := make([]ec2types.InstanceType, 0, len(types))
	for _, t := range types {
		its = append(its, ec2types.InstanceType(t))
	}
	out := map[string]int{}
	p := ec2.NewDescribeInstanceTypesPaginator(c.ec2, &ec2.DescribeInstanceTypesInput{InstanceTypes: its})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, it := range page.InstanceTypes {
			if it.VCpuInfo != nil {
				out[string(it.InstanceType)] = int(aws.ToInt32(it.VCpuInfo.DefaultVCpus))
			}
		}
	}
	return out, nil
}

// EC2Quotas calls servicequotas:ListServiceQuotas for the ec2 service.
func (c *Client) EC2Quotas(ctx context.Context) (map[string]float64, error) {
	out := map[string]float64{}
	p := servicequotas.NewListServiceQuotasPaginator(c.sq, &servicequotas.ListServiceQuotasInput{ServiceCode: aws.String("ec2")})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, q := range page.Quotas {
			out[aws.ToString(q.QuotaName)] = aws.ToFloat64(q.Value)
		}
	}
	return out, nil
}
