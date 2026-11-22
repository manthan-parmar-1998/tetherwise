package safety

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/manthan-parmar-1998/tetherwise/internal/audit"
)

// MutatingActions are the IAM actions Prove asks about.
// They are the writes tetherwise must not be able to perform.
var MutatingActions = []string{
	"cloudwatch:PutMetricData",
	"ec2:AuthorizeSecurityGroupIngress",
	"ec2:RevokeSecurityGroupIngress",
	"es:CreateDomain",
	"es:DeleteDomain",
	"es:UpdateDomainConfig",
	"kafka:CreateCluster",
	"kafka:DeleteCluster",
	"kafka:UpdateClusterConfiguration",
	"logs:CreateLogGroup",
	"logs:DeleteLogGroup",
	"logs:PutRetentionPolicy",
	"rds:CreateDBInstance",
	"rds:DeleteDBInstance",
	"rds:ModifyDBInstance",
	"s3:DeleteBucket",
	"s3:PutObject",
}

// IdentityAPI is sts:GetCallerIdentity.
type IdentityAPI interface {
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

// SimulatorAPI is iam:SimulatePrincipalPolicy.
type SimulatorAPI interface {
	SimulatePrincipalPolicy(ctx context.Context, params *iam.SimulatePrincipalPolicyInput, optFns ...func(*iam.Options)) (*iam.SimulatePrincipalPolicyOutput, error)
}

// Reviewer answers whether a Kubernetes verb is allowed.
type Reviewer interface {
	Allowed(ctx context.Context, group, resource, verb string) (bool, error)
}

// Recorder receives one entry per external read.
type Recorder interface {
	Record(service, operation, target string) error
}

// Options selects which proofs to run.
// Identity and Simulator are set only when AWS credentials are configured.
// Reviewer is set only when a live Kubernetes read will run.
type Options struct {
	IKnow     bool
	Identity  IdentityAPI
	Simulator SimulatorAPI
	Reviewer  Reviewer
	Audit     Recorder
	Actions   []string
}

// Proof is what the startup check was able to show.
type Proof struct {
	Identity           string
	MutatingAllowed    []string
	K8sMutatingAllowed []string
	Warnings           []string
}

type k8sCheck struct {
	group    string
	resource string
	verb     string
}

// workloadReviews are the mutating verbs Prove refuses to hold on workloads.
// SelfSubjectAccessReview itself is not in this list: posting a review is
// how the check asks the question.
var workloadReviews = []k8sCheck{
	{"apps", "deployments", "create"},
	{"apps", "deployments", "update"},
	{"apps", "deployments", "delete"},
	{"apps", "deployments", "patch"},
	{"", "configmaps", "create"},
	{"", "configmaps", "update"},
	{"", "configmaps", "delete"},
	{"", "configmaps", "patch"},
	{"", "pods", "create"},
	{"", "pods", "update"},
	{"", "pods", "delete"},
	{"", "pods", "patch"},
}

// Prove refuses to continue when it can see a mutating permission,
// unless IKnow is set. A failed proof is also a refusal: read-only has
// to be shown, not assumed.
func Prove(ctx context.Context, opt Options) (Proof, error) {
	var proof Proof
	if opt.Identity != nil {
		if err := proveAWS(ctx, opt, &proof); err != nil {
			return proof, err
		}
	}
	if opt.Reviewer != nil {
		if err := proveK8s(ctx, opt, &proof); err != nil {
			return proof, err
		}
	}
	return proof, nil
}

func proveAWS(ctx context.Context, opt Options, proof *Proof) error {
	if err := record(opt.Audit, "sts", "GetCallerIdentity", "caller"); err != nil {
		return err
	}
	out, err := opt.Identity.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return soften(opt.IKnow, proof, "AWS GetCallerIdentity failed; continuing because --i-know was set",
			fmt.Errorf("refusing to run: AWS credentials are configured but sts:GetCallerIdentity failed: %w", err))
	}
	arn := aws.ToString(out.Arn)
	proof.Identity = audit.Redact(arn, true)
	if opt.Simulator == nil {
		return soften(opt.IKnow, proof, "IAM simulation was not available; continuing because --i-know was set",
			errors.New("refusing to run: AWS credentials are configured but iam:SimulatePrincipalPolicy is unavailable, so read-only is not proven"))
	}
	actions := opt.Actions
	if len(actions) == 0 {
		actions = MutatingActions
	}
	allowed, err := simulate(ctx, opt, arn, actions)
	if err != nil {
		return soften(opt.IKnow, proof, "IAM simulation failed; continuing because --i-know was set",
			fmt.Errorf("refusing to run: iam:SimulatePrincipalPolicy failed, so read-only is not proven: %w", err))
	}
	proof.MutatingAllowed = allowed
	if len(allowed) == 0 {
		return nil
	}
	list := strings.Join(allowed, ", ")
	return soften(opt.IKnow, proof, "continuing because --i-know was set; credentials allow mutating actions: "+list,
		fmt.Errorf("refusing to run: AWS credentials allow mutating actions (%s); rerun with --i-know only in a disposable environment", list))
}

func simulate(ctx context.Context, opt Options, arn string, actions []string) ([]string, error) {
	var marker *string
	var allowed []string
	for {
		if err := record(opt.Audit, "iam", "SimulatePrincipalPolicy", audit.Redact(arn, true)); err != nil {
			return nil, err
		}
		out, err := opt.Simulator.SimulatePrincipalPolicy(ctx, &iam.SimulatePrincipalPolicyInput{
			PolicySourceArn: aws.String(arn),
			ActionNames:     actions,
			Marker:          marker,
		})
		if err != nil {
			return nil, err
		}
		for _, result := range out.EvaluationResults {
			if result.EvalDecision == iamtypes.PolicyEvaluationDecisionTypeAllowed {
				allowed = append(allowed, aws.ToString(result.EvalActionName))
			}
		}
		if !out.IsTruncated || out.Marker == nil || *out.Marker == "" {
			break
		}
		marker = out.Marker
	}
	sort.Strings(allowed)
	return allowed, nil
}

func proveK8s(ctx context.Context, opt Options, proof *Proof) error {
	var allowed []string
	for _, check := range workloadReviews {
		target := check.verb + " " + nameOf(check.group, check.resource)
		if err := record(opt.Audit, "kubernetes", "SelfSubjectAccessReview", target); err != nil {
			return err
		}
		ok, err := opt.Reviewer.Allowed(ctx, check.group, check.resource, check.verb)
		if err != nil {
			return soften(opt.IKnow, proof, "Kubernetes access review failed; continuing because --i-know was set",
				fmt.Errorf("refusing to run: SelfSubjectAccessReview failed, so read-only is not proven: %w", err))
		}
		if ok {
			allowed = append(allowed, target)
		}
	}
	sort.Strings(allowed)
	proof.K8sMutatingAllowed = allowed
	if len(allowed) == 0 {
		return nil
	}
	list := strings.Join(allowed, ", ")
	return soften(opt.IKnow, proof, "continuing because --i-know was set; the kube identity allows "+list,
		fmt.Errorf("refusing to run: kube identity allows mutating verbs (%s); rerun with --i-know only in a disposable cluster", list))
}

func nameOf(group, resource string) string {
	if group == "" {
		return resource
	}
	return group + "/" + resource
}

func soften(iKnow bool, proof *Proof, warning string, err error) error {
	if iKnow {
		proof.Warnings = append(proof.Warnings, warning)
		return nil
	}
	return err
}

func record(rec Recorder, service, operation, target string) error {
	if rec == nil {
		return nil
	}
	if err := rec.Record(service, operation, target); err != nil {
		return fmt.Errorf("audit %s %s: %w", service, operation, err)
	}
	return nil
}

// AWSClients loads the default AWS config and the two read clients Prove uses.
func AWSClients(ctx context.Context) (IdentityAPI, SimulatorAPI, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("load aws config: %w", err)
	}
	return sts.NewFromConfig(cfg), iam.NewFromConfig(cfg), nil
}

// CredentialsConfigured reports whether the process has a credential file
// or environment variable that would select an AWS principal.
// It does not contact the instance metadata service.
func CredentialsConfigured() bool {
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" || os.Getenv("AWS_PROFILE") != "" || os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE") != "" ||
		os.Getenv("AWS_CONTAINER_CREDENTIALS_RELATIVE_URI") != "" || os.Getenv("AWS_CONTAINER_CREDENTIALS_FULL_URI") != "" {
		return true
	}
	if path := os.Getenv("AWS_SHARED_CREDENTIALS_FILE"); path != "" {
		if _, err := os.Stat(path); err == nil { //nolint:gosec // G703: AWS credentials file path from the environment
			return true
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	for _, name := range []string{"credentials", "config"} {
		if _, err := os.Stat(filepath.Join(home, ".aws", name)); err == nil {
			return true
		}
	}
	return false
}

// KubeReviewer posts SelfSubjectAccessReviews through Verb("create").
// See the package comment for why this is not a client-go Create call.
type KubeReviewer struct {
	Client kubernetes.Interface
}

// Allowed reports the API server's answer for one verb.
func (k KubeReviewer) Allowed(ctx context.Context, group, resource, verb string) (bool, error) {
	if k.Client == nil {
		return false, errors.New("kubernetes client is nil")
	}
	body := &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Group:    group,
				Resource: resource,
				Verb:     verb,
			},
		},
	}
	var result authorizationv1.SelfSubjectAccessReview
	err := k.Client.AuthorizationV1().RESTClient().Verb("create").
		Resource("selfsubjectaccessreviews").
		Body(body).
		Do(ctx).
		Into(&result)
	if err != nil {
		return false, err
	}
	return result.Status.Allowed, nil
}
