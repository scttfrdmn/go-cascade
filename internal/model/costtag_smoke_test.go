package model

import (
	"context"
	"os"
	"strings"
	"testing"
)

// The one property a fake cannot establish: that a tagged application inference
// profile ARN is actually ACCEPTED by bedrock-runtime converse — for BOTH model
// families, whose source ARNs have different shapes (`us.*` Claude is an
// account-scoped inference-profile ARN, a bare-ID open-weight model an
// account-less foundation-model one). A resolver that produced a plausible ARN
// the runtime rejects would fail every sample, and costtag_test.go cannot see
// that: it asserts what we send, not what Bedrock does with it.
//
// Manual and opt-in, never in CI, because it spends money and mutates the
// account (CreateInferenceProfile). Two guards, both required:
//
//	GO_CASCADE_LIVE_SMOKE=1 AWS_PROFILE=... go test ./internal/model/ -run Smoke -v
//
// Cost is ~$0.01: two 24-token completions. What it does NOT verify is the
// attribution itself — Cost Explorer lags ~1 day, so confirming the spend landed
// under the tag is a next-day query, not an assertion here.
//
// It writes under a dedicated `smoke-test` tag value rather than the default, so
// a probe never lands in a real experiment's cost row.
func TestCostTagLiveSmoke(t *testing.T) {
	if os.Getenv("GO_CASCADE_LIVE_SMOKE") != "1" {
		t.Skip("live smoke test: set GO_CASCADE_LIVE_SMOKE=1 (spends ~$0.01 and creates inference profiles)")
	}
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-west-2"
	}
	ctx := context.Background()

	// One model per ARN shape. Both are configuration and churn — verify with
	// `go-cascade models` and `aws bedrock list-foundation-models` if either fails
	// with a not-found rather than an ARN error.
	cases := []struct {
		family  string
		modelID string
	}{
		{"us.* cross-region inference profile", "us.anthropic.claude-haiku-4-5-20251001-v1:0"},
		{"bare-ID foundation model", "qwen.qwen3-coder-30b-a3b-v1:0"},
	}

	for _, c := range cases {
		t.Run(c.family, func(t *testing.T) {
			b, err := NewBedrock(ctx, region, "smoke-test")
			if err != nil {
				t.Fatalf("NewBedrock: %v", err)
			}
			// Resolution first, on its own, so a resolver failure is distinguishable
			// from a runtime rejection. resolve never returns an error by design — it
			// falls back to the bare ID — so the *absence* of a change is the failure
			// signal, and it will have warned on stderr.
			got := b.prof.resolve(ctx, c.modelID)
			if got == c.modelID {
				t.Fatalf("%s: resolved to the bare model id, so tagging silently degraded; "+
					"see the warning on stderr for the cause", c.modelID)
			}
			if !strings.Contains(got, "application-inference-profile/") {
				t.Fatalf("%s resolved to %q, which is not an application inference profile ARN",
					c.modelID, got)
			}
			t.Logf("%s -> %s", c.modelID, got)

			// The actual point: the runtime accepts that ARN as a ModelId. Kept tiny —
			// this is a call-path check, not a generation check.
			resp, err := b.Generate(ctx, Request{
				ModelID:     c.modelID,
				Messages:    []Message{{Role: RoleUser, Text: "Reply with the single word: ok"}},
				MaxTokens:   24,
				Temperature: 0,
				Purpose:     PurposeCode,
			})
			if err != nil {
				t.Fatalf("converse through the tagged profile failed: %v\n"+
					"the profile ARN resolved but the runtime rejected it, which is exactly "+
					"the failure a fake cannot catch", err)
			}
			if strings.TrimSpace(resp.Text) == "" {
				t.Error("empty completion through the tagged profile")
			}
			// Usage confirms real tokens were billed, which is what there is to attribute.
			if resp.Usage.OutputTokens == 0 {
				t.Error("no output tokens reported; nothing was billed, so nothing was attributed")
			}
			t.Logf("in=%d out=%d text=%q",
				resp.Usage.InputTokens, resp.Usage.OutputTokens, strings.TrimSpace(resp.Text))
		})
	}
}
