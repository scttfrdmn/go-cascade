package model

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrock"
	btypes "github.com/aws/aws-sdk-go-v2/service/bedrock/types"
)

// fakeProfileAPI stands in for the Bedrock control plane. Tests in this repo never
// call real AWS.
type fakeProfileAPI struct {
	mu sync.Mutex

	// system and foundation are the two source populations, which deliberately have
	// different ARN shapes: the study's Claude tiers are account-scoped
	// SYSTEM_DEFINED profiles, the open-weight tiers are account-less foundation
	// models. Both shapes are real, verified against us-west-2.
	system     map[string]string // profile id -> arn
	foundation map[string]string // model id  -> arn

	app []btypes.InferenceProfileSummary // existing APPLICATION profiles

	created   []*bedrock.CreateInferenceProfileInput
	createErr error
	listErr   error

	listCalls, createCalls int
}

func newFake() *fakeProfileAPI {
	return &fakeProfileAPI{
		system: map[string]string{
			"us.anthropic.claude-sonnet-4-5-20250929-v1:0": "arn:aws:bedrock:us-west-2:1234:inference-profile/us.anthropic.claude-sonnet-4-5-20250929-v1:0",
			"us.anthropic.claude-haiku-4-5-20251001-v1:0":  "arn:aws:bedrock:us-west-2:1234:inference-profile/us.anthropic.claude-haiku-4-5-20251001-v1:0",
		},
		foundation: map[string]string{
			"qwen.qwen3-coder-30b-a3b-v1:0": "arn:aws:bedrock:us-west-2::foundation-model/qwen.qwen3-coder-30b-a3b-v1:0",
		},
	}
}

func (f *fakeProfileAPI) ListInferenceProfiles(_ context.Context, in *bedrock.ListInferenceProfilesInput,
	_ ...func(*bedrock.Options),
) (*bedrock.ListInferenceProfilesOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listCalls++
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := &bedrock.ListInferenceProfilesOutput{}
	switch in.TypeEquals {
	case btypes.InferenceProfileTypeApplication:
		out.InferenceProfileSummaries = f.app
	case btypes.InferenceProfileTypeSystemDefined:
		for id, arn := range f.system {
			out.InferenceProfileSummaries = append(out.InferenceProfileSummaries,
				btypes.InferenceProfileSummary{
					InferenceProfileId:  aws.String(id),
					InferenceProfileArn: aws.String(arn),
					Status:              btypes.InferenceProfileStatusActive,
					Type:                btypes.InferenceProfileTypeSystemDefined,
				})
		}
	default:
		// The real API returns ONLY system-defined profiles when unfiltered. The fake
		// reproduces that, because code relying on the convenient behaviour would pass
		// against a lenient fake and silently create a duplicate profile per run live.
		for id, arn := range f.system {
			out.InferenceProfileSummaries = append(out.InferenceProfileSummaries,
				btypes.InferenceProfileSummary{
					InferenceProfileId:  aws.String(id),
					InferenceProfileArn: aws.String(arn),
					Type:                btypes.InferenceProfileTypeSystemDefined,
				})
		}
	}
	return out, nil
}

func (f *fakeProfileAPI) CreateInferenceProfile(_ context.Context, in *bedrock.CreateInferenceProfileInput,
	_ ...func(*bedrock.Options),
) (*bedrock.CreateInferenceProfileOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createCalls++
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.created = append(f.created, in)
	src := in.ModelSource.(*btypes.InferenceProfileModelSourceMemberCopyFrom).Value
	arn := "arn:aws:bedrock:us-west-2:1234:application-inference-profile/" + *in.InferenceProfileName
	f.app = append(f.app, btypes.InferenceProfileSummary{
		InferenceProfileName: in.InferenceProfileName,
		InferenceProfileArn:  aws.String(arn),
		Status:               btypes.InferenceProfileStatusActive,
		Type:                 btypes.InferenceProfileTypeApplication,
		Models:               []btypes.InferenceProfileModel{{ModelArn: aws.String(src)}},
	})
	return &bedrock.CreateInferenceProfileOutput{
		InferenceProfileArn: aws.String(arn),
		Status:              btypes.InferenceProfileStatusActive,
	}, nil
}

func (f *fakeProfileAPI) ListFoundationModels(_ context.Context, _ *bedrock.ListFoundationModelsInput,
	_ ...func(*bedrock.Options),
) (*bedrock.ListFoundationModelsOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := &bedrock.ListFoundationModelsOutput{}
	for id, arn := range f.foundation {
		out.ModelSummaries = append(out.ModelSummaries, btypes.FoundationModelSummary{
			ModelId: aws.String(id), ModelArn: aws.String(arn),
		})
	}
	return out, nil
}

// captureStderr redirects the package's warning sink for one call.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	var sb strings.Builder
	old := stderr
	stderr = &sb
	defer func() { stderr = old }()
	fn()
	return sb.String()
}

// Both model families must resolve, and their ARNs must come from a lookup rather
// than string construction. The two shapes differ — an account-scoped
// inference-profile ARN for the us.* Claude tiers, an account-less
// foundation-model ARN for the bare-ID open-weight tiers — so a resolver that
// assembled either by hand would work on one family and silently fail on the
// other, and the open-weight tier is exactly the cheap tier a cost comparison
// cares about.
func TestResolveBothModelFamilies(t *testing.T) {
	f := newFake()
	r := newResolver(f, DefaultCostTag, "us-west-2")
	ctx := context.Background()

	for _, id := range []string{
		"us.anthropic.claude-sonnet-4-5-20250929-v1:0",
		"qwen.qwen3-coder-30b-a3b-v1:0",
	} {
		got := r.resolve(ctx, id)
		if got == id {
			t.Fatalf("%s: not resolved (still the bare model id) — spend would be untagged", id)
		}
		if !strings.Contains(got, "application-inference-profile/") {
			t.Errorf("%s resolved to %q, want an application inference profile ARN", id, got)
		}
	}
	if len(f.created) != 2 {
		t.Fatalf("created %d profiles, want 2", len(f.created))
	}
	// The tag is the whole point; assert the key and value, not merely that some tag
	// was set. A wrong key means an inactive cost-allocation dimension and no
	// attribution, which looks exactly like no tagging at all.
	for _, in := range f.created {
		if len(in.Tags) != 1 || aws.ToString(in.Tags[0].Key) != CostTagKey ||
			aws.ToString(in.Tags[0].Value) != DefaultCostTag {
			t.Errorf("profile %s tags = %v, want %s=%s",
				aws.ToString(in.InferenceProfileName), in.Tags, CostTagKey, DefaultCostTag)
		}
		// Idempotency must key on the derived name, not a random token, or a concurrent
		// fan-out creates two profiles for one model and splits its spend across two rows.
		if aws.ToString(in.ClientRequestToken) != aws.ToString(in.InferenceProfileName) {
			t.Errorf("ClientRequestToken = %q, want the profile name %q",
				aws.ToString(in.ClientRequestToken), aws.ToString(in.InferenceProfileName))
		}
	}
}

// Resolution is cached per model for the process. Without this a 400-problem run
// makes a control-plane round trip per sample against an API that throttles.
func TestResolveCachesPerModel(t *testing.T) {
	f := newFake()
	r := newResolver(f, DefaultCostTag, "us-west-2")
	ctx := context.Background()
	const id = "us.anthropic.claude-haiku-4-5-20251001-v1:0"

	first := r.resolve(ctx, id)
	for range 20 {
		if got := r.resolve(ctx, id); got != first {
			t.Fatalf("resolve is not stable: %q then %q", first, got)
		}
	}
	if f.createCalls != 1 {
		t.Errorf("createCalls = %d, want 1 (one profile per model, reused)", f.createCalls)
	}
	if f.listCalls > 3 {
		t.Errorf("listCalls = %d; resolution is not cached", f.listCalls)
	}
}

// A second run must find the first run's profile instead of creating another. Two
// profiles for one model split its spend across two Cost Explorer rows, so the
// total stops being readable off one line — the failure is a wrong bill, not an error.
func TestResolveReusesAnExistingProfile(t *testing.T) {
	f := newFake()
	ctx := context.Background()
	const id = "us.anthropic.claude-haiku-4-5-20251001-v1:0"

	first := newResolver(f, DefaultCostTag, "us-west-2").resolve(ctx, id)
	if f.createCalls != 1 {
		t.Fatalf("first run createCalls = %d, want 1", f.createCalls)
	}
	// A fresh resolver is a fresh process: no in-memory cache, only what AWS holds.
	second := newResolver(f, DefaultCostTag, "us-west-2").resolve(ctx, id)
	if second != first {
		t.Errorf("second run resolved to %q, want the existing %q", second, first)
	}
	if f.createCalls != 1 {
		t.Errorf("createCalls = %d after a second run, want 1 — a duplicate profile splits the spend", f.createCalls)
	}
}

// A name-matched profile that wraps a DIFFERENT model must be rejected. Using it
// would send the samples to the wrong model and bill them under another model's
// row: a wrong measurement, not merely a wrong invoice. Falling back untagged is
// the right outcome.
func TestResolveRejectsAProfileWrappingAnotherModel(t *testing.T) {
	f := newFake()
	const id = "us.anthropic.claude-haiku-4-5-20251001-v1:0"
	f.app = []btypes.InferenceProfileSummary{{
		InferenceProfileName: aws.String(profileName(DefaultCostTag, id)),
		InferenceProfileArn:  aws.String("arn:aws:bedrock:us-west-2:1234:application-inference-profile/impostor"),
		Status:               btypes.InferenceProfileStatusActive,
		Type:                 btypes.InferenceProfileTypeApplication,
		// Right name, wrong model.
		Models: []btypes.InferenceProfileModel{{
			ModelArn: aws.String("arn:aws:bedrock:us-west-2::foundation-model/amazon.nova-lite-v1:0"),
		}},
	}}
	r := newResolver(f, DefaultCostTag, "us-west-2")

	var got string
	out := captureStderr(t, func() { got = r.resolve(context.Background(), id) })
	if got != id {
		t.Errorf("resolved to %q, want a fallback to the bare model id", got)
	}
	if !strings.Contains(out, "UNTAGGED") {
		t.Errorf("a mismatched profile must warn that spend is untagged; got %q", out)
	}
}

// A profile that exists but is not ACTIVE cannot serve inference. Falling back is
// right: sending to a pending profile fails the converse call, and a failed sample
// is a lost draw — strictly worse than an untagged one.
func TestResolveRejectsANonActiveProfile(t *testing.T) {
	f := newFake()
	const id = "us.anthropic.claude-haiku-4-5-20251001-v1:0"
	src := f.system[id]
	f.app = []btypes.InferenceProfileSummary{{
		InferenceProfileName: aws.String(profileName(DefaultCostTag, id)),
		InferenceProfileArn:  aws.String("arn:aws:bedrock:us-west-2:1234:application-inference-profile/pending"),
		Status:               "CREATING",
		Type:                 btypes.InferenceProfileTypeApplication,
		Models:               []btypes.InferenceProfileModel{{ModelArn: aws.String(src)}},
	}}
	r := newResolver(f, DefaultCostTag, "us-west-2")
	var got string
	_ = captureStderr(t, func() { got = r.resolve(context.Background(), id) })
	if got != id {
		t.Errorf("resolved to %q on a non-ACTIVE profile, want the bare model id", got)
	}
}

// The whole point of the fallback: a run must not die over cost bookkeeping.
// CreateInferenceProfile is a distinct IAM permission from InvokeModel, so
// credentials that can run the study may well not be able to tag it. It must warn
// ONCE — a per-sample warning would bury the run's real output — and it must not
// retry a call already known to fail.
func TestResolveFallsBackAndWarnsOnceOnDenial(t *testing.T) {
	f := newFake()
	f.createErr = errors.New("AccessDeniedException: not authorized to perform bedrock:CreateInferenceProfile")
	r := newResolver(f, DefaultCostTag, "us-west-2")
	ctx := context.Background()
	const id = "us.anthropic.claude-sonnet-4-5-20250929-v1:0"

	out := captureStderr(t, func() {
		for range 5 {
			if got := r.resolve(ctx, id); got != id {
				t.Fatalf("resolve = %q, want the bare model id as a fallback", got)
			}
		}
	})
	if n := strings.Count(out, "warning:"); n != 1 {
		t.Errorf("warned %d times, want exactly 1 (per model, not per sample): %q", n, out)
	}
	// Silence would be worse than the failure: the run would look tagged and the
	// spend would be unattributable with nothing on the record saying so.
	if !strings.Contains(out, "UNTAGGED") {
		t.Errorf("the warning must say the spend is untagged; got %q", out)
	}
	if f.createCalls != 1 {
		t.Errorf("createCalls = %d, want 1 — a failed resolution must be cached, not retried per sample", f.createCalls)
	}
}

// An unknown model resolves to itself rather than erroring. Model IDs are
// configuration and churn, so a stale one must degrade to untagged spend, not
// break the run.
func TestResolveUnknownModelFallsBack(t *testing.T) {
	f := newFake()
	r := newResolver(f, DefaultCostTag, "us-west-2")
	const id = "us.anthropic.claude-nonexistent-v9:0"
	var got string
	out := captureStderr(t, func() { got = r.resolve(context.Background(), id) })
	if got != id {
		t.Errorf("resolve = %q, want %q", got, id)
	}
	if !strings.Contains(out, "UNTAGGED") {
		t.Errorf("want an untagged warning, got %q", out)
	}
	if f.createCalls != 0 {
		t.Error("must not create a profile for a model that does not exist")
	}
}

// An empty tag disables tagging entirely: identical behaviour to before this
// existed, and no control-plane calls at all. This is what -cost-tag="" buys, and
// what `models` relies on so that asking what exists cannot create something.
func TestEmptyTagDisablesResolutionEntirely(t *testing.T) {
	f := newFake()
	r := newResolver(f, "", "us-west-2")
	const id = "us.anthropic.claude-sonnet-4-5-20250929-v1:0"
	if got := r.resolve(context.Background(), id); got != id {
		t.Errorf("resolve = %q, want the bare id when tagging is off", got)
	}
	if f.listCalls != 0 || f.createCalls != 0 {
		t.Errorf("tagging off must make no control-plane calls; got %d list, %d create",
			f.listCalls, f.createCalls)
	}
	// A nil resolver is the disabled provider, and must be safe to call.
	var nilr *resolver
	if got := nilr.resolve(context.Background(), id); got != id {
		t.Errorf("nil resolver resolve = %q, want %q", got, id)
	}
}

// Profile names must be stable across runs (or every run creates a new profile and
// the spend fragments) and distinct across models (or one model's spend is billed
// under another's).
func TestProfileNameStableAndCollisionResistant(t *testing.T) {
	ids := []string{
		"us.anthropic.claude-sonnet-4-5-20250929-v1:0",
		"us.anthropic.claude-haiku-4-5-20251001-v1:0",
		"us.anthropic.claude-opus-4-5-20251101-v1:0",
		"us.anthropic.claude-sonnet-4-6",
		"us.meta.llama4-maverick-17b-instruct-v1:0",
		"qwen.qwen3-coder-30b-a3b-v1:0",
	}
	seen := map[string]string{}
	for _, id := range ids {
		n := profileName(DefaultCostTag, id)
		if n != profileName(DefaultCostTag, id) {
			t.Fatalf("%s: name is not deterministic", id)
		}
		if prev, dup := seen[n]; dup {
			t.Errorf("%s and %s both map to profile name %q — one model's spend would be billed under the other", prev, id, n)
		}
		seen[n] = id
		// '.' and ':' are not valid in a profile name; the whole call fails if they leak.
		if strings.ContainsAny(n, ".:") {
			t.Errorf("%s -> %q contains a character a profile name rejects", id, n)
		}
		if n == "" || len(n) > 64 {
			t.Errorf("%s -> %q has length %d, want 1..64", id, n, len(n))
		}
	}

	// Truncation alone is not collision-safe: two long IDs sharing a 64-char prefix
	// must still differ, since a name collision is how spend gets misattributed.
	long1 := strings.Repeat("a", 80) + "-one"
	long2 := strings.Repeat("a", 80) + "-two"
	if profileName(DefaultCostTag, long1) == profileName(DefaultCostTag, long2) {
		t.Error("two long model ids collided after truncation")
	}

	// Different tag values must not collide either: -cost-tag is how a run is
	// separated from other traffic, so two tags sharing a profile defeats it.
	if profileName("go-cascade", "m") == profileName("other-project", "m") {
		t.Error("two tag values produced the same profile name")
	}
}

// The two model families' ARN shapes differ in whether an account is present, so
// the model-identity comparison must key on the trailing identifier. A profile
// legitimately lists several regions' ARNs for one model, so full-ARN equality
// would reject a profile we created ourselves.
func TestWrapsModelComparesTheModelIdentifier(t *testing.T) {
	s := btypes.InferenceProfileSummary{Models: []btypes.InferenceProfileModel{
		{ModelArn: aws.String("arn:aws:bedrock:us-east-1::foundation-model/amazon.nova-lite-v1:0")},
		{ModelArn: aws.String("arn:aws:bedrock:us-west-2::foundation-model/amazon.nova-lite-v1:0")},
	}}
	if !wrapsModel(s, "arn:aws:bedrock:us-east-2::foundation-model/amazon.nova-lite-v1:0") {
		t.Error("a third region's ARN for the same model must match")
	}
	if wrapsModel(s, "arn:aws:bedrock:us-west-2::foundation-model/qwen.qwen3-coder-30b-a3b-v1:0") {
		t.Error("a different model must not match")
	}
	if wrapsModel(btypes.InferenceProfileSummary{}, "anything") {
		t.Error("a profile wrapping no models must not match")
	}
}
