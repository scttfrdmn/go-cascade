// Package model abstracts the text-generation backend so the cascade can be
// exercised against Bedrock or against a deterministic mock.
package model

import "context"

// Request is a single generation request.
type Request struct {
	ModelID     string
	System      string
	Messages    []Message
	MaxTokens   int
	Temperature float32
	// Seed disambiguates otherwise identical samples. Bedrock has no seed
	// parameter for Claude, so it is folded into the prompt as a nonce; the
	// mock provider uses it directly.
	Seed int
	// Purpose is trace/mock metadata. Real providers ignore it.
	Purpose Purpose
}

// Purpose labels what a request is for.
type Purpose string

// Request purposes.
const (
	PurposeSpec   Purpose = "spec"   // derive API contract + tests from the problem
	PurposePlan   Purpose = "plan"   // rewrite the problem into an implementation plan (two-stage tier)
	PurposeCode   Purpose = "code"   // write a solution against the contract
	PurposeRepair Purpose = "repair" // fix a solution given verifier diagnostics
	// PurposeJudge asks a model to rule on a candidate's correctness. It backs
	// the judge-oracle comparison arm (paper §5.5c) and is deliberately *not* a
	// verifier stage: a judge verdict is a noisy prediction, not a sound
	// refutation, so it must be modelled as its own oracle rather than added to
	// the ladder. See internal/cascade/judge.go.
	PurposeJudge Purpose = "judge"
)

// Role values.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// Message is one conversational turn.
type Message struct {
	Role string
	Text string
}

// Usage reports token consumption for cost accounting.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// Response is a completed generation.
type Response struct {
	Text  string
	Usage Usage
}

// Provider generates text.
type Provider interface {
	Generate(ctx context.Context, req Request) (*Response, error)
	// Name identifies the backend in traces.
	Name() string
}
