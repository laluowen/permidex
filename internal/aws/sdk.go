package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"golang.org/x/time/rate"
)

// iamRate caps IAM reads per second. It is a starting guess: tune it against real throttling.
const iamRate = 10

// sdkIAM implements PolicyAPI with the AWS SDK, using the default credential chain. Every call waits on the
// limiter first, and the SDK's own retryer backs off on throttling.
type sdkIAM struct {
	client  *iam.Client
	limiter *rate.Limiter
}

// NewPolicyAPI returns a PolicyAPI backed by IAM, using the default AWS credential chain.
func NewPolicyAPI(ctx context.Context) (PolicyAPI, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRetryer(func() aws.Retryer {
			return retry.NewStandard(func(o *retry.StandardOptions) { o.MaxAttempts = 8 })
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	return sdkIAM{client: iam.NewFromConfig(cfg), limiter: rate.NewLimiter(iamRate, iamRate)}, nil
}

func (s sdkIAM) wait(ctx context.Context) error {
	return s.limiter.Wait(ctx)
}

func (s sdkIAM) ListPolicies(ctx context.Context, marker string) ([]PolicyRef, string, error) {
	if err := s.wait(ctx); err != nil {
		return nil, "", err
	}
	in := &iam.ListPoliciesInput{Scope: iamtypes.PolicyScopeTypeAws}
	if marker != "" {
		in.Marker = aws.String(marker)
	}
	out, err := s.client.ListPolicies(ctx, in)
	if err != nil {
		return nil, "", err
	}
	refs := make([]PolicyRef, 0, len(out.Policies))
	for _, p := range out.Policies {
		refs = append(refs, PolicyRef{
			Name:             aws.ToString(p.PolicyName),
			ARN:              aws.ToString(p.Arn),
			DefaultVersionID: aws.ToString(p.DefaultVersionId),
		})
	}
	next := ""
	if out.IsTruncated {
		next = aws.ToString(out.Marker)
	}
	return refs, next, nil
}

func (s sdkIAM) Description(ctx context.Context, arn string) (string, error) {
	if err := s.wait(ctx); err != nil {
		return "", err
	}
	out, err := s.client.GetPolicy(ctx, &iam.GetPolicyInput{PolicyArn: aws.String(arn)})
	if err != nil {
		return "", err
	}
	return aws.ToString(out.Policy.Description), nil
}

func (s sdkIAM) Document(ctx context.Context, arn, versionID string) (string, error) {
	if err := s.wait(ctx); err != nil {
		return "", err
	}
	out, err := s.client.GetPolicyVersion(ctx, &iam.GetPolicyVersionInput{
		PolicyArn: aws.String(arn),
		VersionId: aws.String(versionID),
	})
	if err != nil {
		return "", err
	}
	return aws.ToString(out.PolicyVersion.Document), nil
}
