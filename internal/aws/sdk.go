package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
)

// sdkIAM implements PolicyAPI with the AWS SDK, using the default credential chain.
type sdkIAM struct {
	client *iam.Client
}

// NewPolicyAPI returns a PolicyAPI backed by IAM, using the default AWS credential chain.
func NewPolicyAPI(ctx context.Context) (PolicyAPI, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	return sdkIAM{client: iam.NewFromConfig(cfg)}, nil
}

func (s sdkIAM) ListPolicies(ctx context.Context, marker string) ([]PolicyRef, string, error) {
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
	out, err := s.client.GetPolicy(ctx, &iam.GetPolicyInput{PolicyArn: aws.String(arn)})
	if err != nil {
		return "", err
	}
	return aws.ToString(out.Policy.Description), nil
}

func (s sdkIAM) Document(ctx context.Context, arn, versionID string) (string, error) {
	out, err := s.client.GetPolicyVersion(ctx, &iam.GetPolicyVersionInput{
		PolicyArn: aws.String(arn),
		VersionId: aws.String(versionID),
	})
	if err != nil {
		return "", err
	}
	return aws.ToString(out.PolicyVersion.Document), nil
}
