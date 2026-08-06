package model

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrock"
	btypes "github.com/aws/aws-sdk-go-v2/service/bedrock/types"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	brtypes "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

// stderr is where the provider's operational warnings go. A variable so tests can
// capture them without a live AWS call.
var stderr io.Writer = os.Stderr

// Bedrock is a Provider backed by the Bedrock Converse API.
type Bedrock struct {
	rt      *bedrockruntime.Client
	ctl     *bedrock.Client
	retries int
	// prof resolves logical model IDs to tagged application-inference-profile ARNs
	// for cost attribution. Nil disables it. See costtag.go for why resolution
	// happens here, at the wire, and nowhere upstream.
	prof *resolver
}

// NewBedrock builds a provider using the ambient AWS credential chain.
//
// costTag is the cost-allocation tag value applied to this run's spend (see
// CostTagKey); empty disables tagging and sends bare model IDs, exactly as before
// this existed.
func NewBedrock(ctx context.Context, region, costTag string) (*Bedrock, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	ctl := bedrock.NewFromConfig(cfg)
	b := &Bedrock{
		rt:      bedrockruntime.NewFromConfig(cfg),
		ctl:     ctl,
		retries: 5,
	}
	if costTag != "" {
		b.prof = newResolver(ctl, costTag, region)
	}
	return b, nil
}

// Name implements Provider.
func (b *Bedrock) Name() string { return "bedrock" }

// Generate implements Provider. Throttling is retried with jittered backoff;
// Bedrock throttles aggressively when a cascade fans out samples in parallel.
func (b *Bedrock) Generate(ctx context.Context, req Request) (*Response, error) {
	msgs := make([]brtypes.Message, 0, len(req.Messages))
	for _, m := range req.Messages {
		role := brtypes.ConversationRoleUser
		if m.Role == RoleAssistant {
			role = brtypes.ConversationRoleAssistant
		}
		msgs = append(msgs, brtypes.Message{
			Role:    role,
			Content: []brtypes.ContentBlock{&brtypes.ContentBlockMemberText{Value: m.Text}},
		})
	}

	maxTok := int32(req.MaxTokens)
	temp := req.Temperature
	// The only place a logical model ID becomes a profile ARN. Everything upstream —
	// config.Tier.ModelID, and so every invariant-#3 contamination check — keeps
	// comparing logical IDs. resolve never fails; it falls back to the bare ID.
	sendID := req.ModelID
	if b.prof != nil {
		sendID = b.prof.resolve(ctx, req.ModelID)
	}
	in := &bedrockruntime.ConverseInput{
		ModelId:  &sendID,
		Messages: msgs,
		InferenceConfig: &brtypes.InferenceConfiguration{
			MaxTokens:   &maxTok,
			Temperature: &temp,
		},
	}
	if req.System != "" {
		in.System = []brtypes.SystemContentBlock{
			&brtypes.SystemContentBlockMemberText{Value: req.System},
		}
	}

	var lastErr error
	for attempt := range b.retries {
		if attempt > 0 {
			d := time.Duration(1<<attempt)*250*time.Millisecond +
				time.Duration(rand.N(250))*time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(d):
			}
		}
		out, err := b.rt.Converse(ctx, in)
		if err != nil {
			if isRetryable(err) {
				lastErr = err
				continue
			}
			return nil, fmt.Errorf("bedrock converse (%s): %w", req.ModelID, err)
		}
		return decodeConverse(out)
	}
	return nil, fmt.Errorf("bedrock converse (%s) exhausted retries: %w", req.ModelID, lastErr)
}

func decodeConverse(out *bedrockruntime.ConverseOutput) (*Response, error) {
	msg, ok := out.Output.(*brtypes.ConverseOutputMemberMessage)
	if !ok {
		return nil, errors.New("bedrock: unexpected output shape")
	}
	var sb strings.Builder
	for _, c := range msg.Value.Content {
		if t, ok := c.(*brtypes.ContentBlockMemberText); ok {
			sb.WriteString(t.Value)
		}
	}
	r := &Response{Text: sb.String()}
	if out.Usage != nil {
		if out.Usage.InputTokens != nil {
			r.Usage.InputTokens = int(*out.Usage.InputTokens)
		}
		if out.Usage.OutputTokens != nil {
			r.Usage.OutputTokens = int(*out.Usage.OutputTokens)
		}
	}
	return r, nil
}

func isRetryable(err error) bool {
	var thr *brtypes.ThrottlingException
	if errors.As(err, &thr) {
		return true
	}
	var svc *brtypes.ServiceUnavailableException
	if errors.As(err, &svc) {
		return true
	}
	var ise *brtypes.InternalServerException
	return errors.As(err, &ise)
}

// InferenceProfile is a trimmed view of a Bedrock inference profile.
type InferenceProfile struct {
	ID     string
	Name   string
	Status string
	// Type is SYSTEM_DEFINED (Bedrock's own, what a tier's model_id names) or
	// APPLICATION (created by an account, what cost tagging creates).
	Type string
}

// ListProfiles enumerates inference profiles visible to the account. Model IDs
// churn; this is how you discover the correct ones rather than trusting a
// hardcoded default.
//
// Both types are listed, and that needs saying: ListInferenceProfiles with no
// TypeEquals returns ONLY the SYSTEM_DEFINED ones, so the obvious single call
// omits the account's own application profiles entirely — including the ones cost
// tagging creates, which this command would then report as absent. That is the
// same shape as the standing caveat that this call omits the bare-ID open-weight
// catalog: an absence here is not evidence a profile does not exist.
func (b *Bedrock) ListProfiles(ctx context.Context) ([]InferenceProfile, error) {
	var out []InferenceProfile
	for _, t := range []btypes.InferenceProfileType{
		btypes.InferenceProfileTypeSystemDefined,
		btypes.InferenceProfileTypeApplication,
	} {
		p := bedrock.NewListInferenceProfilesPaginator(b.ctl, &bedrock.ListInferenceProfilesInput{
			TypeEquals: t,
		})
		for p.HasMorePages() {
			page, err := p.NextPage(ctx)
			if err != nil {
				return nil, fmt.Errorf("list %s inference profiles: %w", t, err)
			}
			for _, s := range page.InferenceProfileSummaries {
				ip := InferenceProfile{Status: string(s.Status), Type: string(s.Type)}
				if s.InferenceProfileId != nil {
					ip.ID = *s.InferenceProfileId
				}
				if s.InferenceProfileName != nil {
					ip.Name = *s.InferenceProfileName
				}
				out = append(out, ip)
			}
		}
	}
	return out, nil
}
