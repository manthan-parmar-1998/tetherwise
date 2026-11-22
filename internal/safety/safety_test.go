package safety

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"sigs.k8s.io/yaml"
)

func TestProveReadOnly(t *testing.T) {
	t.Parallel()
	id := &fakeSTS{arn: "arn:aws:iam::123456789012:user/lab"}
	sim := &fakeIAM{decision: iamtypes.PolicyEvaluationDecisionTypeImplicitDeny}
	proof, err := Prove(context.Background(), Options{Identity: id, Simulator: sim, Audit: &memAudit{}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(proof.Identity, "123456789012") {
		t.Fatalf("identity leaked account: %s", proof.Identity)
	}
	if len(proof.MutatingAllowed) != 0 {
		t.Fatal(proof.MutatingAllowed)
	}
	if id.calls != 1 || sim.calls != 1 {
		t.Fatalf("calls id=%d sim=%d", id.calls, sim.calls)
	}
}

func TestProveRefusesMutation(t *testing.T) {
	t.Parallel()
	sim := &fakeIAM{
		decision: iamtypes.PolicyEvaluationDecisionTypeAllowed,
		allowed:  []string{"es:DeleteDomain"},
		more:     true,
	}
	_, err := Prove(context.Background(), Options{
		Identity:  &fakeSTS{arn: "arn:aws:iam::123456789012:role/admin"},
		Simulator: sim,
		Actions:   []string{"es:DeleteDomain", "es:CreateDomain"},
	})
	if err == nil || !strings.Contains(err.Error(), "es:DeleteDomain") {
		t.Fatalf("err = %v", err)
	}
	if sim.calls < 2 {
		t.Fatal("pagination did not continue")
	}
	proof, err := Prove(context.Background(), Options{
		IKnow:    true,
		Identity: &fakeSTS{arn: "arn:aws:iam::123456789012:role/admin"},
		Simulator: &fakeIAM{
			decision: iamtypes.PolicyEvaluationDecisionTypeImplicitDeny,
			allowed:  []string{"s3:PutObject"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(proof.Warnings) == 0 || len(proof.MutatingAllowed) != 1 {
		t.Fatalf("%#v", proof)
	}
}

func TestProveIncomplete(t *testing.T) {
	t.Parallel()
	_, err := Prove(context.Background(), Options{Identity: &fakeSTS{err: errors.New("expired")}})
	if err == nil || !strings.Contains(err.Error(), "GetCallerIdentity") {
		t.Fatalf("err = %v", err)
	}
	proof, err := Prove(context.Background(), Options{IKnow: true, Identity: &fakeSTS{err: errors.New("expired")}})
	if err != nil || len(proof.Warnings) == 0 {
		t.Fatalf("proof=%#v err=%v", proof, err)
	}
	_, err = Prove(context.Background(), Options{Identity: &fakeSTS{arn: "arn:aws:iam::123456789012:user/lab"}})
	if err == nil || !strings.Contains(err.Error(), "SimulatePrincipalPolicy") {
		t.Fatalf("err = %v", err)
	}
	_, err = Prove(context.Background(), Options{
		Identity:  &fakeSTS{arn: "arn:aws:iam::123456789012:user/lab"},
		Simulator: &fakeIAM{err: errors.New("denied")},
	})
	if err == nil {
		t.Fatal("simulation error")
	}
}

func TestProveKubernetes(t *testing.T) {
	t.Parallel()
	reviewer := &fakeReviewer{allowed: map[string]bool{"delete apps/deployments": true}}
	_, err := Prove(context.Background(), Options{Reviewer: reviewer})
	if err == nil || !strings.Contains(err.Error(), "delete apps/deployments") {
		t.Fatalf("err = %v", err)
	}
	proof, err := Prove(context.Background(), Options{IKnow: true, Reviewer: reviewer})
	if err != nil || len(proof.K8sMutatingAllowed) != 1 {
		t.Fatalf("proof=%#v err=%v", proof, err)
	}
	ok, err := Prove(context.Background(), Options{Reviewer: &fakeReviewer{}})
	if err != nil || len(ok.K8sMutatingAllowed) != 0 {
		t.Fatalf("%#v %v", ok, err)
	}
	_, err = Prove(context.Background(), Options{Reviewer: &fakeReviewer{err: errors.New("timeout")}})
	if err == nil {
		t.Fatal("review error")
	}
}

func TestDeployManifests(t *testing.T) {
	t.Parallel()
	policy := readRepo(t, "deploy/iam-policy.json")
	var doc struct {
		Statement []struct {
			Action any `json:"Action"`
		} `json:"Statement"`
	}
	if err := yaml.Unmarshal(policy, &doc); err != nil {
		t.Fatal(err)
	}
	var actions []string
	for _, st := range doc.Statement {
		switch a := st.Action.(type) {
		case string:
			actions = append(actions, a)
		case []any:
			for _, item := range a {
				actions = append(actions, item.(string))
			}
		}
	}
	if len(actions) == 0 {
		t.Fatal("no actions")
	}
	for _, action := range actions {
		for _, prefix := range []string{"Create", "Delete", "Update", "Put", "Modify", "Terminate"} {
			parts := strings.SplitN(action, ":", 2)
			name := parts[len(parts)-1]
			if strings.HasPrefix(name, prefix) {
				t.Fatalf("policy allows %s", action)
			}
		}
	}
	role := readRepo(t, "deploy/clusterrole.yaml")
	if !bytesContains(role, "selfsubjectaccessreviews") || !bytesContains(role, "get") {
		t.Fatal("clusterrole missing expected rules")
	}
	if bytesContains(role, "deletecollection") {
		t.Fatal("clusterrole allows deletecollection")
	}
}

func readRepo(t *testing.T, rel string) []byte {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join("..", "..", rel))
	if err != nil {
		t.Fatal(err)
	}
	return buf
}

func bytesContains(buf []byte, s string) bool {
	return strings.Contains(string(buf), s)
}

type fakeSTS struct {
	arn   string
	err   error
	calls int
}

func (f *fakeSTS) GetCallerIdentity(context.Context, *sts.GetCallerIdentityInput, ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return &sts.GetCallerIdentityOutput{Arn: aws.String(f.arn)}, nil
}

type fakeIAM struct {
	decision iamtypes.PolicyEvaluationDecisionType
	allowed  []string
	err      error
	more     bool
	calls    int
}

func (f *fakeIAM) SimulatePrincipalPolicy(_ context.Context, in *iam.SimulatePrincipalPolicyInput, _ ...func(*iam.Options)) (*iam.SimulatePrincipalPolicyOutput, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	var results []iamtypes.EvaluationResult
	if f.calls == 1 && f.more {
		return &iam.SimulatePrincipalPolicyOutput{IsTruncated: true, Marker: aws.String("next")}, nil
	}
	allowed := map[string]struct{}{}
	for _, name := range f.allowed {
		allowed[name] = struct{}{}
	}
	for _, name := range in.ActionNames {
		decision := f.decision
		if _, ok := allowed[name]; ok {
			decision = iamtypes.PolicyEvaluationDecisionTypeAllowed
		}
		results = append(results, iamtypes.EvaluationResult{EvalActionName: aws.String(name), EvalDecision: decision})
	}
	return &iam.SimulatePrincipalPolicyOutput{EvaluationResults: results}, nil
}

type fakeReviewer struct {
	allowed map[string]bool
	err     error
}

func (f *fakeReviewer) Allowed(_ context.Context, group, resource, verb string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.allowed[verb+" "+nameOf(group, resource)], nil
}

type memAudit struct{ n int }

func (m *memAudit) Record(string, string, string) error {
	m.n++
	return nil
}
