package model

import (
	"context"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrock"
	btypes "github.com/aws/aws-sdk-go-v2/service/bedrock/types"
)

// Cost attribution by application inference profile.
//
// This account is shared: on 2026-08-04 the `Amazon Bedrock Service` line was
// $1126.23, all of it in the untagged bucket, against a whole-study total of
// ~$197. Untagged spend is unrecoverable after the fact — which is why experiment
// 30's ~$1.20 could not be reconciled and `results/scarfree-sweep-n9.md` documents
// a three-filter workaround instead of a figure.
//
// A tagged application inference profile fixes that: its ARN is passed as
// ConverseInput.ModelId and the spend bills against the profile, whose tags
// surface as cost-allocation dimensions.
//
// WHY NOT ConverseInput.RequestMetadata. It is a map[string]string on the struct
// this package already builds, so it looks like the answer. Its own doc says
// "Key-value pairs that you can use to filter invocation logs" — a *logging*
// filter, not a billing dimension, and model invocation logging is not even
// configured on this account. It would have attributed nothing and returned no
// error.
//
// WHY THIS LIVES AT THE PROVIDER BOUNDARY, which is the load-bearing part.
// Router.contaminated (internal/cascade/cascade.go) enforces invariant #3 by
// STRING EQUALITY on model IDs — tier.ModelID == cfg.TestModel — as do the judge
// arm's checks. In the default config that comparison fires today: tier "mid" and
// TestModel are both claude-sonnet-4-5. Substituting a profile ARN anywhere
// upstream of those checks turns one model into two different strings, so
// contaminated() returns false, OracleContaminated never sets, and contaminated
// records enter calibration. No test would fail — the guarded branch simply stops
// running. So config.Tier.ModelID keeps the logical model ID everywhere, and the
// ARN comes into existence only here, one function above the wire.

// profileAPI is the slice of the Bedrock control plane the resolver needs. It
// exists so the resolver is testable against a fake: this repo never calls real
// AWS from a test.
type profileAPI interface {
	ListInferenceProfiles(ctx context.Context, in *bedrock.ListInferenceProfilesInput,
		opts ...func(*bedrock.Options)) (*bedrock.ListInferenceProfilesOutput, error)
	CreateInferenceProfile(ctx context.Context, in *bedrock.CreateInferenceProfileInput,
		opts ...func(*bedrock.Options)) (*bedrock.CreateInferenceProfileOutput, error)
	ListFoundationModels(ctx context.Context, in *bedrock.ListFoundationModelsInput,
		opts ...func(*bedrock.Options)) (*bedrock.ListFoundationModelsOutput, error)
}

// CostTagKey is the tag whose values Cost Explorer breaks spend down by.
//
// Reusing the account's existing key is deliberate. `Project` is the only ACTIVE
// user-defined cost-allocation tag here and already carries 20 values, so no
// activation step is needed. A fresh key would require activation and, worse,
// would not backfill: spend recorded before activation stays unattributed
// forever.
const CostTagKey = "Project"

// DefaultCostTag is the value that identifies this study's traffic.
const DefaultCostTag = "go-cascade"

// resolver turns logical model IDs into tagged application-inference-profile
// ARNs, creating a profile on first use and caching it for the process.
type resolver struct {
	api    profileAPI
	tag    string // cost-allocation tag value; "" disables resolution entirely
	region string

	mu     sync.Mutex
	arns   map[string]string // logical model ID -> ARN to send as ModelId
	warned map[string]bool   // one warning per model, not one per sample
}

func newResolver(api profileAPI, tag, region string) *resolver {
	return &resolver{
		api: api, tag: tag, region: region,
		arns:   map[string]string{},
		warned: map[string]bool{},
	}
}

// resolve returns what to send as ConverseInput.ModelId for a logical model ID.
//
// It NEVER fails: on any error it returns modelID unchanged, warns once, and the
// run proceeds untagged. A run must not die over cost bookkeeping — but it must
// not silently pretend to be tagged either, hence the warning. The fallback is
// reached by an IAM gap in practice: CreateInferenceProfile is a distinct
// permission from bedrock:InvokeModel, so credentials that can run the study may
// well not be able to tag it.
//
// The caching is per process and per logical model ID. Six models across this
// repo's configs means at most six control-plane round trips for a whole run, and
// resolution is skipped entirely when tagging is off.
func (r *resolver) resolve(ctx context.Context, modelID string) string {
	if r == nil || r.tag == "" || modelID == "" {
		return modelID
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if arn, ok := r.arns[modelID]; ok {
		return arn
	}
	arn, err := r.ensureProfile(ctx, modelID)
	if err != nil {
		if !r.warned[modelID] {
			r.warned[modelID] = true
			// Named as a cost-attribution failure, not a model failure, so it is not
			// mistaken for a reason the samples themselves are suspect.
			_, _ = fmt.Fprintf(stderr, "warning: cost tagging unavailable for %s (%v); "+
				"this run's spend will be UNTAGGED and unattributable in Cost Explorer\n", modelID, err)
		}
		// Cache the fallback too: without this, every sample retries a call that is
		// going to fail, which on a 400-problem run is hundreds of pointless
		// round trips against a control-plane API that throttles.
		r.arns[modelID] = modelID
		return modelID
	}
	r.arns[modelID] = arn
	return arn
}

// ensureProfile finds this study's profile for a model, or creates it.
func (r *resolver) ensureProfile(ctx context.Context, modelID string) (string, error) {
	// The source ARN is looked up, never assembled. The two model families in this
	// study have DIFFERENT ARN shapes — us.anthropic.claude-sonnet-4-6 is an
	// account-scoped inference-profile ARN, qwen.qwen3-coder-30b-a3b-v1:0 is an
	// account-less foundation-model ARN — and both are returned verbatim by a list
	// call, so there is no reason to hand-build either and get one wrong. This
	// mirrors the standing rule that Bedrock model IDs are configuration, never
	// constants.
	source, err := r.sourceARN(ctx, modelID)
	if err != nil {
		return "", err
	}
	name := profileName(r.tag, modelID)
	if arn, ok, err := r.findProfile(ctx, name, source); err != nil {
		return "", err
	} else if ok {
		return arn, nil
	}

	out, err := r.api.CreateInferenceProfile(ctx, &bedrock.CreateInferenceProfileInput{
		InferenceProfileName: aws.String(name),
		Description:          aws.String("go-cascade: cost attribution for " + modelID),
		ModelSource:          &btypes.InferenceProfileModelSourceMemberCopyFrom{Value: source},
		Tags:                 []btypes.Tag{{Key: aws.String(CostTagKey), Value: aws.String(r.tag)}},
		// Idempotency: a concurrent fan-out can race two creations for the same
		// model, and this makes the loser a no-op rather than a duplicate profile
		// that splits the model's spend across two rows. The token must be derived
		// from the name, not random, or it defeats its own purpose.
		ClientRequestToken: aws.String(name),
	})
	if err != nil {
		return "", err
	}
	if out.InferenceProfileArn == nil {
		return "", fmt.Errorf("created profile %s has no ARN", name)
	}
	// A profile that is not ACTIVE cannot serve inference. Falling back is right:
	// sending to a pending profile would fail the converse call, and a failed
	// sample is a lost draw — worse than an untagged one.
	if out.Status != "" && out.Status != btypes.InferenceProfileStatusActive {
		return "", fmt.Errorf("profile %s is %s, not ACTIVE", name, out.Status)
	}
	return *out.InferenceProfileArn, nil
}

// findProfile looks for an existing application profile by name, and verifies it
// actually wraps the model we are about to bill against it.
//
// The verification is the point. Reusing a name-matched profile that wraps a
// DIFFERENT model would bill this model's spend under another model's row and
// send the samples to the wrong model — a wrong measurement, not just a wrong
// invoice. On a mismatch we return not-found, which routes to creation and, if
// that collides, to the untagged fallback.
func (r *resolver) findProfile(ctx context.Context, name, source string) (string, bool, error) {
	// APPLICATION is required. With no filter the call returns only the account's
	// SYSTEM_DEFINED profiles, so an unfiltered search would never find our own
	// profile and would create a duplicate on every run.
	p := bedrock.NewListInferenceProfilesPaginator(r.api, &bedrock.ListInferenceProfilesInput{
		TypeEquals: btypes.InferenceProfileTypeApplication,
	})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return "", false, err
		}
		for _, s := range page.InferenceProfileSummaries {
			if s.InferenceProfileName == nil || *s.InferenceProfileName != name {
				continue
			}
			if s.InferenceProfileArn == nil {
				continue
			}
			if !wrapsModel(s, source) {
				return "", false, fmt.Errorf("profile %s exists but does not wrap %s", name, source)
			}
			if s.Status != "" && s.Status != btypes.InferenceProfileStatusActive {
				return "", false, fmt.Errorf("profile %s is %s, not ACTIVE", name, s.Status)
			}
			return *s.InferenceProfileArn, true, nil
		}
	}
	return "", false, nil
}

// wrapsModel reports whether a profile tracks the given source.
//
// The comparison is on the model identity rather than the whole ARN, and that
// needs two normalisations — both of which were found by a LIVE second run, not
// by a fake:
//
// (1) A profile lists one ARN per region it can route to, so full-ARN equality
// would reject a profile we ourselves created.
//
// (2) A profile copied from a CROSS-REGION source lists the UNDERLYING
// foundation models, without the routing prefix. Copying from
// `us.anthropic.claude-haiku-4-5-20251001-v1:0` yields a profile whose models are
// `{us-east-1,us-east-2,us-west-2}::foundation-model/anthropic.claude-haiku-4-5-20251001-v1:0`
// — no `us.`. So comparing the source's tail against the profile's tails never
// matches for any Claude tier, and this failure is invisible on the FIRST run:
// creation returns the ARN directly and never consults findProfile. Every run
// after the first would warn and fall back to untagged, for 91% of real spend.
func wrapsModel(s btypes.InferenceProfileSummary, source string) bool {
	want := modelIdentity(source)
	for _, m := range s.Models {
		if m.ModelArn != nil && modelIdentity(*m.ModelArn) == want {
			return true
		}
	}
	return false
}

// crossRegionPrefixes are Bedrock's inference-routing prefixes. A system-defined
// profile ID is one of these plus the underlying model ID, and the profiles copied
// from it carry the model ID alone, so identity comparison must fold them away.
//
// This is an allowlist rather than "strip the first dotted segment" because a
// bare model ID also starts with a dotted segment — its vendor (`qwen.`,
// `anthropic.`, `amazon.`) — and stripping that could equate two vendors' models
// with the same name. The account in use has both `us.` and `global.` today;
// the others are Bedrock's documented set.
var crossRegionPrefixes = []string{"us-gov.", "us.", "eu.", "apac.", "global."}

func modelIdentity(arn string) string {
	id := arnTail(arn)
	for _, p := range crossRegionPrefixes {
		if strings.HasPrefix(id, p) {
			return id[len(p):]
		}
	}
	return id
}

func arnTail(arn string) string {
	if i := strings.LastIndexByte(arn, '/'); i >= 0 {
		return arn[i+1:]
	}
	return arn
}

// sourceARN finds the ARN of the model or system-defined profile to copy from.
//
// System-defined profiles are searched first because the study's Claude tiers are
// `us.*` cross-region profiles, which are NOT foundation models and do not appear
// in ListFoundationModels. The open-weight tiers (qwen, llama, …) are bare model
// IDs and are found by the second search. Both are needed; neither alone covers
// the configs in this repo.
func (r *resolver) sourceARN(ctx context.Context, modelID string) (string, error) {
	p := bedrock.NewListInferenceProfilesPaginator(r.api, &bedrock.ListInferenceProfilesInput{
		TypeEquals: btypes.InferenceProfileTypeSystemDefined,
	})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return "", err
		}
		for _, s := range page.InferenceProfileSummaries {
			if s.InferenceProfileId != nil && *s.InferenceProfileId == modelID && s.InferenceProfileArn != nil {
				return *s.InferenceProfileArn, nil
			}
		}
	}

	fms, err := r.api.ListFoundationModels(ctx, &bedrock.ListFoundationModelsInput{})
	if err != nil {
		return "", err
	}
	for _, m := range fms.ModelSummaries {
		if m.ModelId != nil && *m.ModelId == modelID && m.ModelArn != nil {
			return *m.ModelArn, nil
		}
	}
	return "", fmt.Errorf("no foundation model or system-defined profile with id %q", modelID)
}

// profileName derives a stable, valid profile name from a tag and a model ID.
//
// Stability is what makes creation idempotent across runs: the same model always
// maps to the same name, so a second run finds the first run's profile instead of
// creating a second one and splitting the model's spend across two rows.
//
// Model IDs carry '.' and ':' (us.anthropic.claude-sonnet-4-5-20250929-v1:0),
// which profile names do not accept, so they are folded to '-'.
func profileName(tag, modelID string) string {
	var sb strings.Builder
	sb.Grow(len(tag) + len(modelID) + 1)
	writeSlug(&sb, tag)
	sb.WriteByte('-')
	writeSlug(&sb, modelID)
	name := sb.String()

	// Cap the length, and make the truncated form collision-resistant. Truncation
	// alone is not safe: two long model IDs sharing a prefix would collapse to one
	// name, and a name collision is how one model's spend ends up billed under
	// another's. The hash is of the full name, so distinct inputs stay distinct.
	const maxName = 64
	if len(name) > maxName {
		h := fnv.New32a()
		_, _ = h.Write([]byte(name))
		suffix := fmt.Sprintf("-%08x", h.Sum32())
		name = name[:maxName-len(suffix)] + suffix
	}
	return name
}

func writeSlug(sb *strings.Builder, s string) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
			sb.WriteByte(c)
		default:
			sb.WriteByte('-')
		}
	}
}
