# Verification-Saturated Cascade Routing

### Cost-Optimal Model Selection under Distribution-Free Risk Control, with an Executable Oracle for Go

**Working paper — July 2026**

---

## Abstract

We consider the problem of routing a query to the least expensive member of a
set of language models that will answer it correctly, subject to a bound on the
probability of returning an incorrect answer. We show that the optimal policy
class is forced rather than chosen: solving the Lagrangian of the constrained
problem by backward induction yields a per-stage threshold rule, and under a
monotone-likelihood-ratio condition on the confidence signal nothing more
expressive than a single scalar threshold per stage can improve on it. We then
supply the thresholds with a finite-sample, distribution-free guarantee using
Learn-then-Test, which requires the confidence signal to be *discriminative* but
not *calibrated*.

The general construction has a load-bearing weakness: the correctness oracle.
When correctness is assessed by a judge model, the false-acceptance rate is
unknown, and recovering true risk from observed risk requires the ground truth
one was trying to avoid — the certificate is circular. We show that specializing
to program synthesis closes this gap. An executed verifier is *sound*: failure
implies incorrectness with probability one. This makes verification a zero-risk
branch of the cascade, so verifier stages can only reduce cost at fixed risk, a
property no confidence-based gate possesses. We further show that the same
soundness argument converts a semantic cache from a *predictor* of transfer into
a *verifier* of it, making the cache an arm of the cascade under a single joint
risk budget rather than an uncontrolled error source in front of it.

We instantiate the design for Go, where the ratio of verification cost to model
cost is approximately $4\times10^{-3}$, and report an implementation
(`gocascade`, 4,328 lines) with measured per-stage costs. Building the system
falsified two of our own design assumptions, which we report as results. We
conclude with an explicit account of what the artifact establishes and — at
greater length — what it does not. The body of that account was written before any
live query; a dated reconciliation (§5.6) then maps seventeen subsequent Bedrock
runs onto it. In brief: the central comparative claim — that an executable oracle
certifies materially lower than a judge — is now **demonstrated at n ≤ 64** (α 0.19
vs 0.30 at n = 64, δ = 0.10) rather than only argued, but **not validated**, the n
being far below the paper's own n ≥ 300 bar and the benchmark not yet standard.
Every cost-saving figure in the earlier informal treatment of this design remains
structural estimation rather than measurement, and should still not be cited.

---

## 1. Introduction

### 1.1 The problem

A deployed system answering a stream of queries with a language model faces a
choice per query: which model. Larger models cost 5–50× more and are more often
correct. Routing every query to the largest is wasteful; routing every query to
the smallest is unreliable. The natural formulation is constrained optimization:

$$\min_{\pi}\ \mathbb{E}\!\left[\mathrm{cost}(\pi)\right]\qquad\text{subject to}\qquad \Pr\!\left[\pi \text{ returns an incorrect answer}\right]\le\alpha. \tag{1}$$

Three things make (1) harder than it looks.

First, the constraint refers to *correctness*, which is not observable at
routing time. Any implementable policy must act on a proxy — a confidence score,
an agreement statistic, a judge's verdict — and the relationship between proxy
and truth is exactly what is unknown.

Second, a guarantee obtained by tuning thresholds on a development set is not a
guarantee. It is an in-sample estimate, and the selection of thresholds by that
same set introduces optimization bias.

Third, systems of this kind are almost always deployed behind a cache, and the
cache silently changes the distribution the router sees. This interaction is
where deployed versions of this idea most often lose whatever guarantee they
started with, and we treat it as a first-class part of the design rather than an
operational detail.

### 1.2 Contributions

1. A derivation showing the threshold policy class is *forced* by the structure
   of (1), not selected for convenience (§2.2–2.3).
2. A distribution-free, finite-sample risk certificate for the resulting
   thresholds via Learn-then-Test, with explicit sample-size requirements
   (§2.5).
3. A monotonicity result for the cache treated as arm zero, stated carefully
   enough to expose the condition under which it holds and the cost term the
   informal version omits (§2.8).
4. The identification of the *cache-miss distribution shift* as the mechanism by
   which caching voids the certificate, and shadow sampling as the correction
   (§2.9).
5. A specialization to program synthesis in which the oracle becomes sound,
   yielding a strictly stronger claim: verification stages reduce cost at fixed
   risk rather than trading against it (§3).
6. An implementation with measured stage costs, and two design corrections
   discovered by building it (§4).
7. An explicit and, we believe, unusually complete account of the artifact's
   evaluative limits (§5).

---

## 2. The general case

### 2.1 Setup and notation

Let $x\sim\mathcal{D}$ be a query drawn from the deployment distribution. Let
$m_1,\dots,m_K$ be models with per-query costs $c_1\le c_2\le\dots\le c_K$. Let

$$Y_k(x)\in\{0,1\}$$

indicate whether $m_k$'s answer to $x$ is correct, according to a fixed oracle
$\mathcal{O}$ whose properties we will scrutinize at length in §3.1. Write
$s_k\in\mathcal{S}\subseteq\mathbb{R}$ for a scalar score observed after
querying $m_k$, and

$$p_k(s)\;=\;\Pr\!\left[Y_k=1\mid s_k=s\right]$$

for the conditional correctness probability given that score.

A *policy* $\pi$ observes $s_1,\dots,s_k$ after $k$ queries and chooses to accept
the current answer or escalate to $m_{k+1}$. At $k=K$ it must accept. This is a
finite-horizon optimal stopping problem with a global chance constraint.

### 2.2 The Lagrangian and backward induction

Introduce a multiplier $\lambda\ge0$ and form

$$L(\pi,\lambda)\;=\;\mathbb{E}\!\left[\mathrm{cost}(\pi)\right]\;+\;\lambda\Big(\Pr[\pi\text{ errs}]-\alpha\Big). \tag{2}$$

For fixed $\lambda$, (2) is an unconstrained stopping problem and can be solved
backward. Define $V_k$ as the optimal expected value-to-go (cost plus
$\lambda$-weighted error) entering stage $k$ *before* the score is revealed, and
$V_k(s)$ after.

At the terminal stage the policy must accept, so

$$V_K(s)\;=\;\lambda\big(1-p_K(s)\big). \tag{3}$$

At stage $k<K$, with score $s$ in hand, the two actions have values

$$
\underbrace{\lambda\big(1-p_k(s)\big)}_{\text{accept}}
\qquad\text{versus}\qquad
\underbrace{c_{k+1}+\mathbb{E}_{s'}\!\left[V_{k+1}(s')\right]}_{\text{escalate}} .
$$

Writing $W_{k+1}:=c_{k+1}+\mathbb{E}_{s'}[V_{k+1}(s')]$ for the cost of
continuing — a quantity that does *not* depend on $s$ — the optimal action is

$$\boxed{\ \text{accept at stage }k \iff \lambda\big(1-p_k(s)\big)\;\le\;W_{k+1}\ } \tag{4}$$

equivalently

$$p_k(s)\;\ge\;1-\frac{W_{k+1}}{\lambda}\;=:\;\pi^\star_k. \tag{5}$$

### 2.3 Why one threshold per stage is optimal

Equation (5) says: accept when the conditional correctness probability exceeds a
stage-specific constant. To convert this into a threshold *on the observed
score*, we need $p_k$ to be monotone in $s$.

> **Condition (MLR).** For each $k$, the family of conditional densities of
> $s_k$ given $Y_k$ has a monotone likelihood ratio: $f(s\mid Y=1)/f(s\mid Y=0)$
> is non-decreasing in $s$.

**Proposition 1.** *Under (MLR), $p_k(s)$ is non-decreasing in $s$, and the
optimal accept region at stage $k$ is an interval of the form
$\{s:s\ge\tau_k\}$.*

*Proof.* By Bayes,
$p_k(s)=\big[1+\tfrac{\Pr[Y=0]}{\Pr[Y=1]}\cdot\tfrac{f(s\mid Y=0)}{f(s\mid Y=1)}\big]^{-1}$,
which is non-decreasing in $s$ exactly when the likelihood ratio is. Combining
with (5), the accept set $\{s:p_k(s)\ge\pi^\star_k\}$ is an up-set in $s$, hence
an interval $[\tau_k,\infty)$. $\square$

The practical consequence is worth stating plainly. **Nothing more expressive
than a single scalar threshold per stage buys anything.** A learned router over
the score, a neural policy, a decision tree on score features — under (MLR) all
are dominated by, or equal to, a threshold. Effort is therefore better spent
making the score more discriminative (improving the MLR ordering) than making
the policy more flexible.

The economic reading of (4) is the one to carry around:

$$\text{escalate}\iff\frac{\Delta p}{\Delta c}\;>\;\frac{1}{\lambda},$$

i.e. escalate when the marginal correctness purchased per marginal dollar
exceeds $1/\lambda$. The multiplier $\lambda$ is the shadow price of correctness.

### 2.4 Duality, randomization, and the exactness caveat

Solving (2) for fixed $\lambda$ gives a policy; recovering the solution to the
*constrained* problem (1) requires choosing $\lambda$ so the constraint binds.
Over deterministic threshold policies this may be impossible: as $\tau_k$ sweeps
its range, the achievable risk moves in jumps wherever the score distribution has
atoms, and the achievable $(\text{cost},\text{risk})$ set is non-convex. A
duality gap results.

The standard repair is randomization at the boundary: accept with probability
$\gamma_k$ when $s=\tau_k$. This convexifies the achievable set and restores
strong duality, exactly as randomized tests do in the Neyman–Pearson lemma.

> **Caveat.** Our implementation does *not* randomize at the boundary. It uses
> deterministic thresholds on a grid. The resulting policy is therefore feasible
> but not exactly optimal; it is the cheapest grid point that certifies. The gap
> is bounded by the grid resolution and, in practice, by the granularity of the
> score. We regard closing this as low-value engineering and high-value only if
> one wants to claim exact optimality, which we do not.

### 2.5 Supplying the thresholds with a guarantee

Choosing $\tau=(\tau_1,\dots,\tau_{K-1})$ by minimizing empirical risk on a
development set yields no guarantee: the selection is data-dependent and the
in-sample risk is optimistically biased. We instead use **Learn-then-Test**
(LTT), which converts threshold selection into a multiple-hypothesis-testing
problem.

Let $\Lambda$ be a finite grid of candidate threshold vectors. Let
$Z_1,\dots,Z_n$ be calibration observations, and let

$$\ell(\tau;Z)\in[0,1]$$

be the loss — here, the indicator that the policy with thresholds $\tau$ returns
an incorrect answer on $Z$. Write $R(\tau)=\mathbb{E}[\ell(\tau;Z)]$ and
$\hat R(\tau)=\frac1n\sum_i\ell(\tau;Z_i)$.

For each $\tau\in\Lambda$ pose the null hypothesis

$$H_\tau:\quad R(\tau)>\alpha .$$

A valid $p$-value for $H_\tau$ is the **Hoeffding–Bentkus** bound:

$$p^{\mathrm{HB}}_\tau=\min\Big\{\underbrace{\exp\big(-n\,h_1(\hat R\wedge\alpha,\ \alpha)\big)}_{\text{Hoeffding}},\ \underbrace{e\cdot\Pr\big[\mathrm{Bin}(n,\alpha)\le\lceil n\hat R\rceil\big]}_{\text{Bentkus}}\Big\} \tag{6}$$

where

$$h_1(a,b)=a\log\frac{a}{b}+(1-a)\log\frac{1-a}{1-b}$$

is the KL divergence between $\mathrm{Bernoulli}(a)$ and
$\mathrm{Bernoulli}(b)$. The minimum of two valid bounds is valid; the Hoeffding
term is tighter in the tails and the Bentkus term near the boundary.

Applying a family-wise error rate (FWER) controlling procedure at level $\delta$
over $\{p_\tau^{\mathrm{HB}}\}_{\tau\in\Lambda}$ and returning any rejected
$\tau$ gives:

> **Theorem 2 (LTT guarantee).** *Let $\hat\tau$ be any threshold vector whose
> null is rejected by an FWER-controlling procedure at level $\delta$. If the
> calibration data are exchangeable with the test point and $\ell\in[0,1]$, then*
> $$\Pr\!\left[R(\hat\tau)\le\alpha\right]\;\ge\;1-\delta,$$
> *where the probability is over the draw of the calibration set.*

Two properties deserve emphasis.

**The score need not be calibrated.** Nothing in Theorem 2 requires $s_k$ to be
a probability, or to be well-calibrated, or to be unbiased. It requires only
that thresholding it produces a policy whose risk can be estimated. Calibration
is supplied *externally* by the conformal procedure. This is liberating in
practice: a cheap discriminative signal beats an expensive calibrated one,
because the expensive calibration is redundant.

**Multiplicity control must not depend on the data.** We implement two options.
*Bonferroni* rejects $H_\tau$ when $p_\tau\le\delta/|\Lambda|$ and is valid under
any ordering. *Fixed-sequence testing* walks a pre-specified ordering of
$\Lambda$ and stops at the first failure to reject; it is more powerful, but is
valid only if the ordering is chosen without reference to the data. We order by
descending threshold magnitude (most conservative first), which is a function of
$\Lambda$ alone. Ordering by observed risk — the tempting choice — would be
data-dependent and would invalidate the FWER control.

#### 2.5.1 Sample size is the binding constraint

Setting $\hat R=0$ in (6), the Hoeffding term reduces to

$$p^{\mathrm{HB}}=\exp\big(-n\,h_1(0,\alpha)\big)=\exp\big(n\log(1-\alpha)\big)=(1-\alpha)^n,$$

which is exactly the probability of observing zero errors when the true risk sits
at $\alpha$. Requiring $(1-\alpha)^n\le\delta$ gives

$$\boxed{\ n\;\ge\;\frac{\log\delta}{\log(1-\alpha)}\ } \tag{7}$$

as the *minimum* calibration set size, achieved only under a flawless policy.

| $\alpha$ | $\delta$ | minimum $n$ (zero errors) |
|---|---|---|
| 0.20 | 0.10 | 11 |
| 0.15 | 0.10 | 15 |
| 0.10 | 0.10 | 22 |
| 0.05 | 0.10 | 45 |
| 0.05 | 0.05 | 59 |
| 0.01 | 0.05 | 299 |

This table is the single most practically important consequence of the
framework, and it is independent of model quality. A tight risk target requires
calibration problems whether or not the models are good, and a system reporting
$\alpha=0.01$ from 50 calibration problems is reporting something other than a
bound.

### 2.6 Ordering when models are not nested

Cheapest-first ordering is optimal when model quality is nested — when a more
expensive model is correct on a superset of the queries a cheaper one gets right.
This is false in practice: small specialists beat large generalists on some
slices.

The correct abstraction is Weitzman's *Pandora's box*. Each model $k$ is a box
with opening cost $c_k$ and a reward distribution. Define the reservation value
$z_k$ as the solution to

$$\mathbb{E}\big[(V_k-z_k)^+\big]=c_k, \tag{8}$$

open boxes in decreasing order of $z_k$, and stop as soon as the best reward in
hand exceeds the next unopened box's index. Weitzman's index policy is exactly
optimal — under independence.

> **Honest limitation.** Model errors are strongly *positively correlated*: hard
> queries are hard for everyone. Pandora's box with correlated rewards is
> APX-hard in general, so no efficient exactly-optimal ordering exists. Constant-
> factor approximations are available. Our implementation sidesteps this by
> assuming a nested cost ordering, which is a modelling choice we do not defend
> beyond convenience; conditioning the index on a coarse query cluster is the
> obvious refinement and is not implemented.

### 2.7 The multi-objective extension

Additional objectives — complexity, latency, memory, allocation counts — can be
stacked. Two structural points govern how.

**Use $\varepsilon$-constraints, not weighted sums.** Scalarizing to
$\sum_j w_jf_j$ and sweeping $w$ traces only the convex hull of the Pareto
frontier; points in non-convex regions are unreachable at *every* weight vector.
The $\varepsilon$-constraint form — bound all objectives but one, optimize the
remainder — reaches the entire frontier. The risk-constrained form (1) is
already an $\varepsilon$-constraint, so the framework is strictly more expressive
than weight tuning, and adding objectives should preserve that form.

**The price of an objective depends on how it is established, not on what it
measures.**

| Class | Examples | Risk-budget cost |
|---|---|---|
| Deterministic | cyclomatic complexity, allocations/op, import set | **zero** — it is a measurement |
| Stochastic | wall-clock latency, race coverage | own risk term, own $\alpha_j$ |
| Judged | maintainability, API taste | inherits the judge's noise floor |

Only the first class is free. LTT extends natively to vector-valued risk with
FWER control across constraints, but each added constraint shrinks the feasible
set and increases the calibration sample requirement.

### 2.8 The cache as arm zero

The conventional deployment places a semantic cache *in front of* the router: on
a new query, retrieve the nearest cached query by embedding similarity, and if
$\cos(x_{\text{new}},x_{\text{cached}})\ge\theta$, return the cached answer. This
is an uncontrolled error source. The quantity that matters is

$$\Pr\!\left[a_{\text{cached}}\text{ is correct for }x_{\text{new}}\right],$$

which is not $\cos(x_{\text{new}},x_{\text{cached}})$ and is not monotone in it.

The correct treatment is to model the cache as **arm zero of the cascade**: a
tier with cost $c_0$ and its own gate, calibrated inside the same LTT procedure
as every other tier. This yields:

> **Proposition 3 (cache monotonicity).** *Let $\Pi_\alpha$ be the set of
> policies over $m_1,\dots,m_K$ with risk $\le\alpha$, and $\Pi'_\alpha$ the
> corresponding set when a cache arm with cost $c_0$ and gate $g$ is prepended,
> where $g$ may always reject. Then*
> $$\min_{\pi\in\Pi'_\alpha}\mathbb{E}[\mathrm{cost}]\;\le\;c_0+\min_{\pi\in\Pi_\alpha}\mathbb{E}[\mathrm{cost}].$$
> *If in addition $g$ is sound — it accepts only answers verified correct — then
> the risk of the cache-augmented policy is no greater than that of the base
> policy, with no additional risk term.*

*Proof.* Any $\pi\in\Pi_\alpha$ embeds into $\Pi'_\alpha$ as the policy that
consults arm zero and always rejects, then executes $\pi$; its cost is
$c_0+\mathbb{E}[\mathrm{cost}(\pi)]$ and its risk is unchanged. Minimizing over
the larger set can only improve. Soundness of $g$ gives
$\Pr[\text{accept at arm }0\wedge Y=0]=0$, so no risk is added. $\square$

Two corrections to the informal version of this claim are worth recording.

1. The cost inequality carries an additive $c_0$. The colloquial statement "the
   cache provably cannot hurt" is exactly true for **risk** and true for **cost**
   only up to the consultation cost. Since in our setting $c_0\sim10^{-5}$ and
   $c_1\sim10^{-4}$, the term is small but it is not zero, and a system that
   consults an always-missing cache does pay for it.
2. The risk claim requires *soundness* of $g$, not merely accuracy. A predictive
   gate with 99% precision adds a risk term; a sound gate adds none. This is the
   hinge on which §3 turns.

### 2.9 The cache-miss distribution shift

This is, in our view, the most commonly overlooked failure in deployed systems of
this shape, and it silently voids Theorem 2.

A warm cache absorbs the head of the query distribution. Let
$\mathcal{H}_t\subset\mathcal{X}$ be the set of queries the cache serves at time
$t$. The router downstream therefore observes

$$\mathcal{D}\mid x\notin\mathcal{H}_t,$$

which is not $\mathcal{D}$; and $\mathcal{H}_t$ grows as the cache warms, so the
shift is non-stationary. If calibration was performed on $\mathcal{D}$ — or,
worse, on traffic collected *behind* an already-warm cache — the calibration
points are not exchangeable with the test points, Theorem 2's hypothesis fails,
and the certificate means nothing.

**Correction: shadow sampling.** Route a fraction $\varepsilon$ of traffic past
the cache, selected by a Bernoulli draw independent of $x$. The resulting stream
is distributed exactly as $\mathcal{D}$, restoring exchangeability. Calibrate on
it. The cost is $\varepsilon$ times the difference between full-cascade and
cache-hit cost, which for $\varepsilon=0.05$ is small.

**For ongoing drift**, layer adaptive conformal inference on the static bound:

$$\tau_{t+1}=\tau_t+\eta\big(\alpha-\mathrm{err}_t\big), \tag{9}$$

which guarantees long-run coverage $\big|\frac1T\sum_t\mathrm{err}_t-\alpha\big|=O\!\big(\tfrac{1}{\eta T}\big)$
under *arbitrary* distribution shift, with no exchangeability assumption. The
division of labour: LTT provides the deploy-time guarantee, (9) tracks the world.
Our implementation performs shadow sampling but does **not** implement (9).

---

## 3. Specialization: program synthesis

### 3.1 The oracle collapse

Everything in §2 is conditional on the oracle $\mathcal{O}$ that defines
$Y_k(x)$. In the general case $\mathcal{O}$ is a judge model, and this is where
the framework leaks.

Let $V$ denote the verifier's verdict and $Y$ the truth. Characterize the
verifier by two error rates:

$$\eta_{\mathrm{fa}}=\Pr[V=1\mid Y=0]\quad\text{(false acceptance)},\qquad \beta=\Pr[V=0\mid Y=1]\quad\text{(false rejection)}.$$

The observed pass rate $P_{\mathrm{obs}}=\Pr[V=1]$ relates to the true pass rate
$P=\Pr[Y=1]$ by

$$P_{\mathrm{obs}}=P(1-\beta)+(1-P)\,\eta_{\mathrm{fa}},$$

so recovering $P$ requires inverting

$$P=\frac{P_{\mathrm{obs}}-\eta_{\mathrm{fa}}}{1-\beta-\eta_{\mathrm{fa}}}. \tag{10}$$

**With a judge model, both $\eta_{\mathrm{fa}}$ and $\beta$ are unknown, and
estimating them requires labelled ground truth — precisely what the judge was
introduced to avoid.** The certificate is circular. In practice this caps
honest certification at the judge's noise floor, commonly cited in the range of
10–15% for code correctness.

Now specialize to program synthesis, where the verifier is *execution*: compile
the candidate and run a test suite. Then

$$V=0\ \Longrightarrow\ Y=0 \tag{11}$$

with probability one. A compilation failure is not *evidence* of incorrectness;
it *is* incorrectness. Consequently $\beta=0$ and (10) collapses to

$$P=\frac{P_{\mathrm{obs}}-\eta_{\mathrm{fa}}}{1-\eta_{\mathrm{fa}}},\qquad \text{true risk among accepted}=\Pr[Y=0\mid V=1]. \tag{12}$$

A single unknown remains: $\eta_{\mathrm{fa}}$, the probability that a wrong
program passes the suite. This is the **oracle gap**, and unlike a judge's error
rate it is *directly estimable* — see §3.7.

Three structural consequences follow, and they are the entire justification for
the specialization.

1. **The failure branch is free.** Since $V=0\Rightarrow Y=0$, a refutation
   contributes nothing to the risk budget. No threshold, no calibration, no
   probability.
2. **Adding verifier stages is monotone in cost.** More verification can only
   move candidates from "accepted" to "refuted", never the reverse. It therefore
   reduces risk weakly and can only reduce cost at fixed risk. **This is not true
   of any confidence-based gate**, where a stricter gate trades false rejections
   against false acceptances.
3. **The cache gate becomes sound.** A retrieved solution can be *executed*
   against the new query's tests rather than predicted to transfer. Proposition
   3's soundness hypothesis is satisfied by construction, and the cache adds no
   risk term. A key collision costs one wasted verification, not a wrong answer.

### 3.2 The verifier ladder

Verification is itself a cascade, and admits the same marginal analysis.
Measured costs on a warm build cache, single vCPU (Xeon @ 2.8 GHz), Go 1.26.5:

| Stage | Refutes | Cost |
|---|---|---|
| $V_0$ parse | syntax | ~0.1 ms (in-process) |
| $V_0'$ import filter | non-stdlib dependencies | free (AST) |
| $V_1$ typecheck | **invented APIs, wrong signatures** | **~1 ms** (in-process) |
| $V_2$ build | code generation | 43 ms |
| $V_3$ vet | oracle/solution mismatch, `lostcancel`, `copylocks` | 113 ms |
| $V_4$ test (visible) | functional defects | 120 ms |
| $V_5$ race | happens-before violations | **1373 ms** |
| $V_6$ bench | allocation ceiling | opt-in |
| $V_A$ accept (hidden) | held-out functional defects | 120 ms |

Stages are ordered by measured cost and executed until first refutation. Two
orderings here are counterintuitive and were determined empirically (§4.3).

$V_5$ is gated on a free AST predicate — the candidate contains `go`, `chan`,
`select`, or imports `sync`/`sync/atomic`. Because the gate is deterministic,
skipping costs nothing against the risk budget. This matters because the race
stage is **32× the cost of a plain test run**.

### 3.3 Why Go, quantitatively

The economics of a verification-saturated design depend entirely on the ratio

$$\rho\;=\;\frac{c_{\mathrm{verify}}}{c_{\mathrm{model}}}.$$

At roughly 800 output tokens, a frontier-model call costs $\sim10^{-2}$ USD and a
small-model call $\sim10^{-4}$ USD. A one-second compile-and-test cycle on a
shared core costs $\sim2\times10^{-5}$ USD. Hence

$$\rho\;\approx\;\frac{2\times10^{-5}}{5\times10^{-3}}\;\approx\;4\times10^{-3}.$$

A verifier stage pays for itself if it changes the escalation decision more often
than $\rho$ — about one query in 250.

**This is a Go argument, not a general one.** Go's compile-speed design goal is
what puts $\rho$ here. In Rust or C++ the equivalent stage runs 10–60 s and lands
near the cost of a *mid-tier model call*, forcing a different topology: many
cheap `check`-equivalents, few full builds, and verification that must be
rationed rather than saturated. In Python, the absence of a static stage removes
$V_1$–$V_3$ entirely and pushes all discrimination onto tests. We claim the
design generalizes; we do not claim the economics do.

Go contributes three further specifics:

- Unused imports and unused variables are **hard compile errors**, converting the
  most common class of model slop into a 43 ms refutation rather than a silent
  defect.
- `go/types` refutes hallucinated APIs — the dominant LLM code failure mode —
  without invoking the compiler.
- A shared, warmed source importer amortizes standard-library typechecking across
  candidates, reducing $V_1$ from 237 ms cold to ~1 ms warm.

### 3.4 Three actions

Compiler and test output constitute a large, free information gain, so the action
space at each stage becomes $\{\text{accept},\text{repair},\text{escalate}\}$.
The rule generalizes (4) to a comparison of marginal slopes:

$$\max\left(\frac{\Delta p_{\mathrm{repair}}}{c_{\mathrm{repair}}},\ \frac{\Delta p_{\mathrm{escalate}}}{c_{k+1}}\right)\ \gtrless\ \frac{1}{\lambda}. \tag{13}$$

Repair dominates early because $c_{\mathrm{repair}}\approx c_k\ll c_{k+1}$ and
the diagnostic localizes the defect precisely. Repair depth is capped at 2, on
the grounds that repair attempts on a fixed model are strongly positively
correlated — a model that cannot fix a defect in two rounds will not fix it in
five. Diagnostics accumulate and are carried forward, so the expensive tier
begins informed rather than cold.

> **Discipline.** The repair loop consults the *visible* test partition only.
> Repairing against the held-out partition would destroy the holdout that makes
> acceptance meaningful.

### 3.5 Behavioural clustering

For general text, self-consistency sampling is expensive relative to its value,
and we argued in the general case that a trained probe on the response dominates
it. Code inverts this, because execution is cheap.

Sample $n$ candidates from a tier, execute all of them, and cluster by the
**observed per-test outcome vector** rather than by source text. Two
implementations that agree on every observable outcome belong to one behavioural
class regardless of how differently they are written; two that differ on any test
are separated. Refuted candidates are keyed by the stage that refuted them, so
they do not pool into a spurious majority.

The routing score is derived from the mass of the largest verified class. §4.3
reports why the *raw* mass is unusable and what replaces it.

### 3.6 Oracle independence

Soundness of the verifier is necessary but not sufficient; the tests must also
be independent of the code. We enforce four mechanisms:

1. **Temporal separation.** The API contract and both test partitions are
   generated *before* any solution exists.
2. **Author separation.** Tests are written by a designated model distinct from
   every code tier. If the accepting tier and the test author coincide, the run
   is flagged `oracle_contaminated` and excluded from calibration.
3. **Partition holdout.** `TestV*` (visible) drives repair; `TestH*` (hidden) is
   the acceptance oracle and never enters a prompt.
4. **No holdout shopping.** When a candidate fails acceptance, the router
   *escalates* rather than trying another candidate from the same tier.
   Repeatedly testing candidates against the held-out partition until one passes
   is selection against the holdout and inflates true risk relative to the
   certificate. This is deliberate and it costs money.

### 3.7 Certifiability, and the honest status of mutation score

From (12), the residual is $\eta_{\mathrm{fa}}=\Pr[Y=0\mid V=1]$: the probability
a wrong program passes the suite. We estimate it by **mutation analysis** —
inject syntactic defects into the accepted program and measure the fraction the
suite kills:

$$M=\frac{\#\{\text{mutants killed}\}}{\#\{\text{mutants that compile}\}}.$$

Mutants that fail to compile are excluded rather than counted as killed, since a
non-compiling mutant is not evidence about the test suite.

This is the step that makes the code cascade certifiable below a judge's noise
floor, and it is also the step most in need of qualification:

> **$M$ is a proxy for $1-\eta_{\mathrm{fa}}$ with unknown bias, not an unbiased
> estimator of it.** The mutation operators are syntactic and local (operator
> swaps, condition negation, increment inversion). The defect distribution of a
> language model is neither. It includes whole-algorithm errors, which are
> *easier* to kill than local mutants, and specification misreadings, which are
> *harder* — a model that solves the wrong problem correctly will pass every
> mutant-derived estimate of suite strength while failing the actual
> requirement. The direction of the net bias is not known to us and we do not
> claim it is small.

**Update (2026-08-01): the direction has since been measured, on one benchmark.**
The §5.5(5) estimator test has now been run live (§5.6): measured
$\eta_{\mathrm{fa}}=0/144$ (95% upper bound 0.021) against a pooled $1-M=0.0996$
that predicted ≈ 11 false acceptances. On that benchmark the net bias is therefore
**conservative** — $M$ under-states $1-\eta_{\mathrm{fa}}$, so using it errs toward
over-estimating risk rather than under-estimating it. Two qualifications keep the
paragraph above standing rather than superseding it: the result is one benchmark
(64 single-file stdlib problems), and it says nothing about whether $M$ *ranks*
candidates by $\eta_{\mathrm{fa}}$ — with zero events in both the high-$M$ and
low-$M$ buckets that question is untouched. So the claim "we do not claim the bias
is small" is now sharper in one direction and unchanged in the other: the bias is
not small, it is large and safe.

A further floor: an assertion-free suite still kills some mutants, because
`i++ → i--` hangs and `< → <=` panics, and a crash is detection. Measured on our
implementation, a suite that asserts nothing scored $M=0.30$ against $M=0.90$ for
a real suite. Absolute values of $M$ are therefore not interpretable; only
comparisons between suites on the same program are.

---

## 4. Implementation

### 4.1 Architecture

`gocascade` — 4,328 lines of Go excluding tests, 1,214 lines of tests, Go 1.26,
AWS Bedrock via the Converse API.

```
cmd/gocascade        CLI: solve, calibrate, models, cache
internal/cascade     router: arm zero, sampling, repair, escalation, speculation
internal/verify      verifier ladder, ephemeral workspaces, mutation analysis
internal/cluster     behavioural clustering, Wilson lower bound
internal/cache       arm zero: solutions, specs, failures
internal/calibrate   LTT, Hoeffding–Bentkus, certificates
internal/model       Bedrock provider; deterministic mock
internal/prompt      two-phase prompting and reply parsing
internal/config      tiers, cost model, risk knobs
```

Calibration profiles **every tier on every problem** rather than running the
cascade. Because all tiers are recorded, any threshold vector — and any $\alpha$
— can be replayed offline without further model queries. Running the cascade
during calibration would only ever observe the tiers the current thresholds
happened to reach.

Two topologies are implemented. Under a dollar objective the router is a
sequential cascade. Under a latency bound (`--deadline`) it switches to
**speculative parallel** execution: all tiers start concurrently and the
cheapest tier whose candidate survives both partitions wins. This is not a
tuning difference. Dollars and latency rank the verifier ladder in opposite
directions — dollars say *verify aggressively, escalate rarely*; latency says
*stop at $V_1$ and escalate immediately* — because parallelism buys latency at
the price of dollars.

### 4.2 Cost accounting

Model cost is computed from reported token usage against per-tier prices.
Verifier cost is wall-clock stage time multiplied by a configured
USD-per-core-second. Both are reported per query, separately, alongside the risk
statement. Runs without a valid certificate are labelled `UNCERTIFIED` and the
tool declines to state a bound.

### 4.3 Two design corrections discovered by building it

We report these as results because both falsified assumptions we had held
confidently, and neither was discoverable by reasoning alone.

**(a) Raw cluster mass is not a usable routing score.**

On the first calibration run the procedure refused to certify at any $\alpha$,
with a floor of 27.8% empirical risk. Inspecting the recorded observations showed
why: the middle tier reported a score of exactly $1.0$ on all 18 problems while
being wrong 39% of the time. With two samples, a defect invisible to the visible
partition clusters *with* the correct solution, so the mass is $1.0$
unconditionally. The score carried no information, and (MLR) failed outright —
there is no threshold on a constant.

The correction is to report a **Wilson lower confidence bound** on the class
mass rather than the raw fraction:

$$\mathrm{LCB}(k,n,z)=\frac{\hat p+\frac{z^2}{2n}-z\sqrt{\frac{\hat p(1-\hat p)}{n}+\frac{z^2}{4n^2}}}{1+\frac{z^2}{n}},\qquad \hat p=k/n. \tag{14}$$

Unanimity among two samples and among five are both mass $1.0$, but they are not
the same evidence:

| verified / sampled | raw mass | reported score, $z=1.645$ |
|---|---|---|
| 1 / 1 | 1.00 | 0.27 |
| 2 / 2 | 1.00 | 0.42 |
| 5 / 5 | 1.00 | 0.65 |
| 10 / 10 | 1.00 | 0.79 |

This makes the statistic monotone in *evidence* rather than in sample luck, so a
tier that has not earned confidence escalates. Empirical risk fell from 0.278 to
0.000 on the same data.

The general lesson: (MLR) is not a formality to be assumed. It is a property of a
specific score that can fail, and when it fails the framework correctly refuses
to certify rather than silently degrading.

**(b) The verifier ladder's cost ordering is not the conventional one.**

`go build` does not compile `_test.go` files. A mismatch between the solution and
the oracle therefore survives the build stage and is caught by `go vet`, which is
the cheapest stage that typechecks the tests *against* the solution. And `vet`
(113 ms) costs more than `build` (43 ms), so build must run first — the reverse
of the conventional pipeline order. Separately, a shared warmed `go/types` source
importer is ~40× cheaper than spawning `go build` (1 ms vs 43 ms) while still
refuting hallucinated APIs.

---

## 5. What the artifact does and does not establish

This section is deliberately the most detailed in the paper. The design above
makes strong-sounding claims, and the implementation supports some of them and
not others. Conflating the two would be the principal way to misuse this work.

### 5.1 What is established

**The statistical primitives are correct.** The regularized incomplete beta
implementation of the binomial tail agrees with direct summation to $10^{-9}$
across $n\in\{1,5,20,100,500\}$, $p\in\{0.01,0.05,0.25,0.5,0.9\}$. The
Hoeffding–Bentkus $p$-value reduces to the closed form $(1-\alpha)^n$ at zero
observed errors, is monotone in $n$ and in $\hat R$, and returns 1 when
$\hat R\ge\alpha$. These are unit-tested.

**The calibrator refuses when it should.** Given records in which the top tier is
always wrong, no certificate is issued and the failure is explained. Given
records where every oracle shares an author with the code, all records are
excluded and the run aborts with a diagnostic. Given $n=18$ and $\alpha=0.10$,
the certificate is withheld ($p=0.1501$) despite zero empirical errors, becoming
valid at $\alpha=0.15$ ($p=0.0537$) — consistent with (7) to the decimal.

**The verifier ladder is sound on a defect taxonomy.** Syntax errors,
hallucinated stdlib APIs, non-stdlib imports, solution/oracle signature
mismatches, and wrong answers are each refuted, each at the expected stage.
A non-strict-comparison defect that passes the visible partition is caught by
the held-out partition. A data race is caught by $V_5$ and the concurrency
predicate does not fire on sequential code.

**Stage costs are measured**, not assumed, on stated hardware.

**The oracle-integrity mechanisms are enforced mechanically**, not by convention:
contamination detection, partition separation, and escalation-instead-of-shopping
are all implemented and tested.

### 5.2 What is demonstrated but not validated

**The end-to-end cascade runs**, exercising spec generation, cache consultation,
sampling, behavioural clustering, repair with depth exhaustion, escalation,
acceptance on the held-out partition, mutation analysis, cache admission, and
certificate reporting. Every one of these paths has been observed executing
against real compilation and real test execution.

**But it has only ever been driven by a mock provider.** This is the single most
important limitation in this document.

### 5.3 What is *not* established

> **Superseded in part — read with §5.6.** This subsection is the pre-live record
> (no query had been sent to a production model when it was written). Several items
> below have since been addressed by the live evaluation; §5.6 marks which, and which
> still stand. The text is retained unedited as the honest account of the pre-live
> state.

**No evaluation against a production language model has been performed.** Not
one query has been sent to Bedrock. The Bedrock provider compiles and is
structurally exercised, but it is unexercised against the live API.

**Every behavioural number in §4.3 is a property of our mock, not of any model.**
The mock's defect distribution — which failure modes appear, at what rate, at
which tier — was *stipulated by us*. Consequently:

- The escalation rates observed are artifacts of that stipulation.
- The finding that "the mid tier scored 1.0 while being wrong 39% of the time"
  is a real property of *raw cluster mass under a two-sample tier with an
  invisible defect*, and we believe it generalizes because the mechanism is
  structural. But the specific 39% is ours, not a measurement of any model.
- The certificate at $\alpha=0.15$ is a demonstration that the machinery
  produces certificates. It is **not** a bound on the risk of any real system.

**The comparative cost figures from the informal design discussion were
estimates.** An earlier treatment of this design presented a table suggesting a
general judge-scored cascade would cost ~32 units against ~12.6 for an
execution-scored Go cascade — roughly 2.5×. Those escalation probabilities were
structural estimates derived from assumed tier accuracies, not measurements.
They should not be cited as results. Nothing in the implementation confirms or
refutes them.

**No baseline comparison exists.** We have not compared against (i) always
routing to the largest model, (ii) existing cascade routers, (iii) simple
self-consistency voting, or (iv) a judge-scored variant of this same
architecture. Without (iv) in particular, the central claim — that an executable
oracle certifies lower than a judge — is argued analytically (§3.1) but not
demonstrated empirically.

**No standard benchmark has been used.** The 18 calibration problems are
paraphrases of 4 templates, all single-file, standard-library-only, and small.
HumanEval-Go, MBPP-Go, or a repository-level benchmark would each stress
different parts of the design, and none has been run.

**The $n=18$ calibration is far too small to mean anything** beyond exercising
the code path, as the framework itself correctly reports.

**Timing generalization is unverified.** All measurements are single-vCPU. The
economics change with parallel verification: with $P$ cores the wall-clock cost
of verification falls but the dollar cost does not, which shifts the
latency-bounded topology and not the dollar-bounded one. We have not measured
this.

**Cost-model fidelity is unverified.** Verifier cost is wall-clock × a configured
rate; we have not validated that this tracks actual cloud billing under
contention, nor that reported token counts match invoiced tokens.

### 5.4 Threats to validity

**Construct validity.** Mutation score is the estimator for the oracle gap, and
§3.7 argues it is biased by an unknown amount. If mutation score systematically
overestimates suite strength against LLM-generated defects, the reported oracle
gap is optimistic and the risk statement inherits that optimism. This is the
weakest link in the chain from measurement to claim.

**Internal validity.** The mock is written by the same author as the system under
test, using the same mental model of what defects look like. It is circular in
exactly the way a benchmark should not be. Its function is to exercise code
paths, and we have tried to confine our claims to that.

**External validity.** Restriction to single-file, stdlib-only Go removes
dependency resolution, build configuration, multi-package refactoring, and API
design — most of what makes real code generation hard. Whether behavioural
clustering, repair economics, or the ladder ordering survive that transition is
unknown.

**Statistical validity.** Theorem 2 assumes exchangeability. Shadow sampling
restores it against the cache, but not against (i) temporal drift in the query
stream, (ii) model version changes, which invalidate every cached observation,
or (iii) the fact that the *same* problems are used to select thresholds and are
then paraphrased into the deployment stream.

**A residual we cannot close.** ThreadSanitizer is sound but incomplete: it
observes only executed interleavings. A latent happens-before violation can
survive $V_5$ at any `-count`, and the certificate will not see it. For
concurrency-heavy work this is the dominant unmodelled risk, and the right
response is a separate conformal group for concurrency tasks, which we have not
implemented.

### 5.5 What would constitute validation

> **Partially addressed — read with §5.6.** Arm (c), the judge-scored variant that
> §5.3 flagged as the missing comparison, has since been run (at n ≤ 64); the full
> experiment below — n ≥ 300 on a standard benchmark with all five arms and both
> secondary tests — remains unrun. §5.6 maps the live runs onto this list item by item.

For the record, the experiment we would want:

1. **Benchmark.** 400+ Go problems spanning difficulty, at least a third
   concurrency-involving, with reference implementations and independently
   authored test suites. Split into calibration ($n\ge300$, satisfying (7) for
   $\alpha=0.05$) and evaluation.
2. **Arms.** (a) frontier model always; (b) this cascade with execution oracle;
   (c) this cascade with a judge oracle, identical otherwise; (d) cheapest model
   always; (e) self-consistency at matched cost.
3. **Primary outcomes.** Realized risk on the evaluation split against the
   certified $\alpha$; total cost per query; and for (b) vs (c), the *lowest*
   $\alpha$ each can certify at fixed $\delta$ and $n$.
4. **Secondary.** Sensitivity of realized risk to cache warmth with and without
   shadow sampling — the direct test of §2.9.
5. **Oracle-gap validation.** Correlation between mutation score and *measured*
   $\eta_{\mathrm{fa}}$ on problems where reference implementations permit
   ground-truth labelling. This is the test that would tell us whether §3.7's
   estimator is usable. *(Run at n=64 on 2026-08-01 — see §5.6: the bias is
   measured and conservative; the ranking question needs the n ≥ 300 of (1).)*

Until at least (3) is run, the correct characterization of this work is: a
design with a proof, an implementation with measured components, and no
empirical validation of its central comparative claim.

### 5.6 Reconciliation with the live evaluation (added 2026-08-01)

§5.1–§5.5 were written before a single query had been sent to a production model.
That is no longer true. Seventeen live experiments have since been run against AWS
Bedrock (Claude Haiku/Sonnet/Opus tiers as the cascade, a separate Sonnet as oracle
author, and a non-Claude cheap tier — Llama 4 Maverick — in the cost experiments);
all records and per-experiment write-ups are in `results/`. This subsection
reconciles those runs with the claims above. It **supersedes specific sentences in
§5.3, §5.5, and §7 where they assert no live run exists**, but it does not soften the
parts of §5.3–§5.4 that remain true; where a limitation still stands, it is repeated
here rather than quietly dropped. The pre-live text is left intact above as the
honest record of what was and was not known before the evaluation.

**The overriding caveat, stated once and load-bearing for everything below.** The
live calibration sets reached **n = 28 and n = 64** — far below the **n ≥ 300** that
§5.5(1) names as the bar for an α = 0.05 certificate to *mean* something, and the
benchmark is still the small, single-file, stdlib-only family §5.3 and §5.4 (external
validity) criticize, now expanded to 64 problems across three families
(`problems`/`hard`/`scale`) but not a standard benchmark. **No claim below should be
read as the §5.5 validation experiment.** What the runs establish is narrower and
specific: the mechanisms behave on live models the way the design predicts, and two
of the paper's hedges can be tightened while two others must be *widened*.

**§3.1 / §5.3 central comparative claim — now demonstrated at this scale, not merely
argued.** §5.3 said the claim that an executable oracle certifies lower than a judge
was "argued analytically (§3.1) but not demonstrated empirically," because arm (c) —
a judge-scored variant of the identical architecture — had never been run. It has now
been run on identical candidates (the `--compare` / `--oracle=judge` path). The
executable oracle certified a strictly lower risk bound than the judge at both scales,
and the gap widened with n: **α 0.27 vs 0.32 at n = 28; 0.19 vs 0.30 at n = 64**
(δ = 0.10). β = 0 held on every run — the execution arm's realized (ground-truth)
risk equalled its empirical risk every time, so (11)/(12) are not just a construction.
**Refinement the paper underweights:** the certification gap is driven *mostly by the
judge's false-rejection rate* (β > 0 inflating its empirical risk), not by the
false-acceptance rate η_fa that §3.1 foregrounds. The η_fa danger is *real but
narrow* — see the next point.

**§3.1 judge noise floor — the mechanism was observed, and localized.** §3.1 asserts a
judge caps honest certification at its noise floor (cited 10–15%). Live, the judge
arm's lowest empirical risk was ~0.077–0.085 while its *realized* risk on the same
candidates ran higher (e.g. 0.22 against 0 empirical in the mock-scale judge study;
0.000 realized against 0.073–0.085 empirical on several live draws where the judge
merely over-rejected). The one confirmed case of the dangerous direction — a judge
*accepting* wrong code (η_fa > 0) — was a **scar-free data race**: a defect a reader
cannot see, caught by the executable oracle under `V_5`. For logic defects and
races-with-a-visible-tell, a strong judge was reliable and strictness-robust (37/37
such seeded defects caught, permissive setting included). So §3.1's floor is real, but
its bite is concentrated exactly where the paper says execution's soundness matters:
reading-invisible defects.

**A validation-relevant hazard the paper does not discuss: oracle *unsoundness* from
generated tests.** §3.6/§5.1 treat the oracle as sound by construction (execution ⇒
β = 0). That holds for the *verifier ladder*, but when the **test suite itself is
model-generated** (the spec model authoring `TestH*`/`TestV*`), a suite that refutes a
correct reference is an *unsound* oracle whose labels are noise — a failure mode
orthogonal to η_fa and not modelled in §3. This was observed live and was initially
misread as model error: an early run's "~11% accuracy floor" turned out to be **~93%
spec-model test noise**, not tier error. The fix now in the artifact
(`calibrate -refs <dir>`, optionally `-pin-api`) runs each problem's execution-validated
reference through the generated tests and *excludes* records whose oracle is refuted by
its own reference; with the gate, genuine tier error on the clean n = 64 set was
**0/52**. This is a genuine addition to §3's account of what can go wrong with the
oracle, and it is now enforced mechanically alongside the §5.1 integrity mechanisms.

**Cost — §5.3's "no baseline comparison exists" is closed; the result is mixed and
tuning-dependent.** Arms (a) always-frontier and (d) always-cheapest are now computed
per run. The cascade's cost verdict is *not* uniformly favorable and depends on
fan-out and operating point: at the default 5:2:1 fan-out the cascade *loses* to
always-frontier; at 1:1:1 and with a cheap non-Claude bottom tier at 2:1:1/3:2:1 it
beats always-frontier by ~2.75–3.4× at matched risk. The informal ~2.5× figure §5.3
disavows should still not be cited — the measured advantage exists but is a property of
tuning, not of the design as such. Crucially, at the **deployable α = 0.05** the cost
win is *fragile*: certified thresholds frequently collapse to full escalation, and
across six 5:1:1 draws only **2 certified with a cost win** — and both of those rode
the oracle-soundness exclusion above rather than cheap-tier robustness. The honest
cost claim is therefore: *the cascade can beat always-frontier at matched risk, but a
deployable-α cost win is draw-dependent at this n, and its binding constraint is
cheap-tier confident-error rate, not fan-out* (an analytic dichotomy — fan-out buys
discrimination headroom against *flaky* errors and none against *confident* ones — is
in `results/headroom_theorem.py`).

**Two design levers the paper does not analyze, tested and reported negative.** A
cheaper/more-accurate bottom tier is the cost lever that works (above). A
*generalist-instructs-specialist* two-stage tier (a strong model plans, a cheap model
codes from the plan) was tested with both an Opus and a Haiku planner: it is an
**accuracy lever, not a cost lever** — the plan nudges cheap-tier accuracy up slightly
(0.88 → ~0.92–0.95, noise-band) and does not fix confident errors, while a planner call
on every cheap-tier query makes the cascade 2.1–3.1× *pricier* than always-frontier.
Neither is claimed in §1.2; both are reported so the design space is mapped honestly.

**What still stands from §5.3–§5.5, unchanged.** The n is still far below §5.5(1);
the benchmark is still not a standard one (HumanEval-Go/MBPP-Go/repo-level unrun);
§5.4's external-validity restriction to single-file stdlib-only Go is unchanged;
§5.5(4) (cache-warmth sensitivity, the direct §2.9 test) has **not** been run.
Adaptive conformal inference (9), non-nested ordering (§2.6), and boundary
randomization (§2.4) remain unimplemented. So the correct one-line update to the
closing sentence of §5.5 is: *arm (c) has now been run at n ≤ 64 and the central
comparative claim is demonstrated at that scale; the full §5.5 experiment — n ≥ 300 on
a standard benchmark with all five arms and the two secondary tests — remains unrun.*

**§5.5(5) has now been run, and it *widens* one caveat while closing another**
(`results/estimator-test-n64-2026-08-01.md`, 2026-08-01). At n = 64 × 3 tiers, with
mutation score measured against the *generated* suite and correctness against each
problem's *human-authored canonical* suite (so the estimate is not circular),
measured η_fa was **0/144** (95% Clopper–Pearson upper bound 0.021) while the pooled
mutation gap 1 − M = 0.0996 predicted ≈ 11 false acceptances — a discrepancy with
probability ≈ 2 × 10⁻⁶ under the per-row gaps if M were tight. **So §3.7's "unknown
bias" now has a measured direction on this benchmark: M under-states 1 − η_fa
substantially, i.e. it errs conservatively**, which is the safe direction for a proxy
in a risk argument and is worth stating because §3.7 could not previously say which
way it erred. What the run does **not** settle is whether M *ranks* candidates by
η_fa: M spanned 0.50–1.00, but with zero events in both the M ≥ 0.90 (n=94) and
M < 0.90 (n=40) buckets the bounds overlap entirely, and the twelve lowest-M rows
were all canonically correct (small mutant pools on short functions;
timing-dependent mutants a concurrency suite cannot deterministically kill). The
ordering question needs the n ≥ 300 of §5.5(1). The run also surfaced a hazard class
§3 does not model: the generated oracle's *observed* errors were **all**
over-rejections — 11 confirmed rejections of canonically-correct candidates (7.1% of
labeled rows) against 0 false acceptances, plus 37 rows where no candidate survived
the ladder at all. A generated suite that is sound but *stricter than canonical*
costs escalations, not risk, so it is invisible to the certificate and visible only
in the cost column.

---

## 6. Related work

The components are individually known. Model cascades and confidence-based
routing are established; the contribution here is not the cascade. Learn-then-Test
and conformal risk control provide the distribution-free machinery of §2.5.
Adaptive conformal inference under arbitrary shift provides (9). Weitzman's index
solves the non-nested ordering problem, with hardness results for the correlated
case. Semantic caching for LLM serving is widely deployed, typically with the
embedding-similarity gate we argue against in §2.8. Mutation testing dates to the
1970s and its use as a test-suite adequacy measure is standard, as is the
critique that mutant distributions are unrepresentative. Execution-based
verification and repair-from-diagnostics are both active areas in program
synthesis.

The composition we would tentatively claim as novel is **sound refutation as a
zero-risk cascade branch**: the observation that when the verifier satisfies
(11), verification stages can only reduce cost at fixed risk and never trade
against it, which fails for every confidence-based gate; together with
cache-as-arm-zero under a single joint risk budget, and the cache-miss
exchangeability correction. We have not conducted a systematic literature review
and would want one before asserting priority.

---

## 7. Conclusion

The policy shape for cost-optimal routing under a risk constraint is forced, not
designed: threshold-per-stage is optimal under a monotone-likelihood condition,
and effort belongs in the score rather than the policy. Distribution-free
finite-sample guarantees for those thresholds are available and cheap, and they
require the score to be discriminative rather than calibrated. The binding
practical constraint is calibration sample size, given in closed form by (7) and
independent of model quality.

The general construction's weak point is the oracle, and specializing to program
synthesis repairs it: execution is sound, so refutation is free, verification
becomes a monotone improvement rather than a trade, the cache becomes a verifier
rather than a predictor, and the single remaining unknown — the oracle gap — is
at least estimable rather than circular. Go makes this economically attractive at
$\rho\approx4\times10^{-3}$; other languages will need different topologies.

Building the system falsified two of our assumptions, which is the strongest
argument we can make for building systems rather than only describing them. As
originally written this section closed by noting the system had not been run
against a production model and that the central comparative claim — that an
executable oracle certifies materially lower than a judge — remained argued rather
than demonstrated. That is no longer the case: it has since been run against AWS
Bedrock, and on identical candidates the executable oracle certified a strictly
lower risk bound than a judge at both scales tested (α 0.19 vs 0.30 at n = 64,
δ = 0.10), with the gap widening in n. **The claim is now demonstrated at n ≤ 64 —
not validated:** that n is far below the n ≥ 300 the paper's own bar (§5.5) requires,
and the benchmark is not yet a standard one. §5.6 reconciles the live runs with §5
claim by claim, including two hedges that tighten and two that must widen.

---

## Appendix A: Notation

| Symbol | Meaning |
|---|---|
| $\mathcal{D}$ | deployment query distribution |
| $c_k$ | cost of model $k$ |
| $Y_k$ | correctness indicator for model $k$ |
| $s_k$ | scalar routing score at stage $k$ |
| $p_k(s)$ | $\Pr[Y_k=1\mid s_k=s]$ |
| $\tau_k$ | accept threshold at stage $k$ |
| $\lambda$ | shadow price of correctness (USD per unit error probability) |
| $\alpha$ | target risk |
| $\delta$ | certificate confidence parameter |
| $n$ | calibration set size |
| $\hat R(\tau)$ | empirical risk of threshold vector $\tau$ |
| $h_1(a,b)$ | KL divergence between $\mathrm{Bernoulli}(a)$ and $\mathrm{Bernoulli}(b)$ |
| $\eta_{\mathrm{fa}}$ | verifier false-acceptance rate (the oracle gap) |
| $\beta$ | verifier false-rejection rate; zero for a sound verifier |
| $M$ | mutation score |
| $\rho$ | ratio of verification cost to model cost |
| $\varepsilon$ | shadow-sampling rate |

## Appendix B: Reproducing the reported measurements

Stage costs (§3.2) were obtained on a single-vCPU Intel Xeon @ 2.8 GHz with
Go 1.26.5, using a persistent `GOCACHE`, taking the minimum of five runs after
warming each command. Cold-cache figures differ by an order of magnitude and are
not representative of steady state; an early version of the implementation
created a per-process build cache and paid ~30 s on the first candidate, which
is itself worth noting as an implementation hazard.

The calibration figures (§4.3, §5.1) come from 18 paraphrased problems over 4
templates with the mock provider, `-delta 0.10`, `-step 0.1`, fixed-sequence
multiplicity control. Records are replayable offline via
`gocascade calibrate -from-records`.

The **live** figures cited in §5.6 come from the seventeen Bedrock experiments in
`results/`; each `results/*.md` write-up carries its exact command line and commits
its execution/judge records (replayable with the same `-from-records` path). The
mock numbers above and the live numbers in §5.6 are kept separate on purpose — a
mock number is never a measurement of model behaviour.
