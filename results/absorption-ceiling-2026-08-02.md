# §5.5(4) is not runnable on this benchmark, and that is decidable for free — 2026-08-02

§5.5(4) — **cache-warmth sensitivity**, the direct test of §2.9 — was the last of the
paper's two secondary experiments left unrun (§5.5(5), the §3.7 estimator test, ran as
[experiment 19](estimator-test-n64-2026-08-01.md)). It was scoped and approved to run
live at ~$7 on the MultiPL-E Go benchmark.

**It cannot produce a result there, and no live run was needed to establish that.**
`results/absorption_ceiling.py` measures the question offline and exactly:

```
retrieval candidacy at min_sim=0.35   464/488  (95.1%)
absorption ceiling                      2/488  ( 0.4%)
```

Those are the same benchmark, three orders of magnitude apart. The gap is the finding.

## What the two numbers are, and why only one of them matters

**Candidacy** is text-only: how many problems have some *prior* problem above
`cache_min_similarity`. Retrieval is Jaccard overlap on character trigrams after
`NormalizeProblem` (`internal/cache/canonical.go`) — deliberately not an embedding.

**Absorption** is what a warm cache would actually serve. Arm zero re-executes a
retrieved solution against the **new** query's tests and keeps it only if it passes
(invariant #5: *cache hits are verified, never predicted*). So absorption additionally
requires the donor's solution to satisfy the recipient's suite.

§2.9's effect is a **distribution shift**: a warm cache absorbs the head of the query
stream, so calibrating behind it breaks exchangeability. That mechanism is driven by
absorption, not candidacy. A cache that absorbs 0.4% of traffic shifts nothing
measurable. There is no effect here to be sensitive to, so a paid run would have
reported a **null caused by the benchmark rather than by the method** — the most
expensive kind of uninformative result, and one easy to misread as evidence that §2.9
does not bite.

## The ceiling is exact, not sampled

Two free filters make exhaustive measurement cheap:

1. A donor whose **signature** differs from the recipient's cannot compile against the
   recipient's suite, so it is a guaranteed refutation. `manifest.json` pins every
   signature, so this filter costs nothing and reduces 118,828 candidate pairs to
   **5 groups / 10 problems / 10 ordered transfers**.
2. Every one of those 10 transfers is **executed**. No sampling.

```
  refuted  he_56_correct_bracketing  -> he_61_correct_bracketing
  refuted  he_61_correct_bracketing  -> he_56_correct_bracketing
  ABSORB   mbpp_102_snake_to_camel   -> mbpp_411_snake_to_camel
  ABSORB   mbpp_411_snake_to_camel   -> mbpp_102_snake_to_camel
  refuted  mbpp_267_square_Sum       -> mbpp_287_square_Sum
  refuted  mbpp_287_square_Sum       -> mbpp_267_square_Sum
  refuted  mbpp_554_Split            -> mbpp_629_Split
  refuted  mbpp_629_Split            -> mbpp_554_Split
  ABSORB   mbpp_591_swap_List        -> mbpp_625_swap_List
  ABSORB   mbpp_625_swap_List        -> mbpp_591_swap_List
```

4 of 10 pass, forming **2 bidirectional pairs**: `mbpp_102`/`mbpp_411` (snake_to_camel)
and `mbpp_591`/`mbpp_625` (swap_List). Both are upstream MBPP shipping one task twice
under different ids, with reworded statements and different test tables — exactly the
case a verified cache *should* serve.

It is a **ceiling**, not a realized rate, for three reasons stated in the script: donors
are *reference* solutions (a real cache stores whatever the router accepted, which is at
best as good as the reference), arrival order is ignored (a cache only absorbs a query
whose donor arrived first), and the spec phase runs *before* `tryCache`
(`cascade.go:171` vs `:181`), so even a hit pays for spec generation.

## The stronger result: similarity is anti-correlated with transferability at the top

The top of the similarity distribution is dominated by **antonym pairs**:

| similarity | pair | why it cannot transfer |
|---|---|---|
| **0.949** | `mbpp_404_minimum` ~ `mbpp_309_maximum` | highest pair of all 118,828 — opposite answers |
| 0.937 | `first_Digit` ~ `last_Digit` | opposite ends |
| 0.920 | `even_position` ~ `odd_position` | opposite parity |
| 0.915 | `remove_lowercase` ~ `remove_uppercase` | opposite case |
| 0.913 | `is_nonagonal` ~ `is_num_decagonal` / `is_octagonal` | different polygon |
| 0.908 | `pairs_sum_to_zero` ~ `triples_sum_to_zero` | 2 vs 3 |
| 0.904 | `square_nums` ~ `cube_nums` | exponent |

Near-identical text, opposite required answers. So on this benchmark similarity is not
merely a **weak** predictor of transferability — at the high end it is **anti-correlated**:
a similarity-threshold cache with no re-execution would be *most confidently wrong
exactly where it is most sure*.

The sharpest case is the highest-similarity **same-signature** pair,
`he_56` ~ `he_61` `CorrectBracketing` at **0.836**. Same signature, so it is retrieved,
compiles, and runs — and it is **refuted**: one problem is about `<>`, the other about
`()`. That is invariant #5's argument on real data rather than asserted. A threshold
cache returns a wrong answer there; re-execution catches it.

`top_pairs` also doubles as an **audit of the signature pre-filter**: nothing it drops
from the top of the distribution could have absorbed. Every diff-sig pair above is a
guaranteed compile failure, so the filter buys exactness, not a shortcut.

## Guard on the measurement itself

The Python port of `cache.Similarity` is **cross-checked against the Go original** on
sampled pairs and hard-fails on any disagreement (`check_port`, verified exact on 6
pairs). Without that, the 95.1% candidacy figure would be this script's artifact rather
than the router's behaviour. A first draft omitted `NormalizeProblem` (lowercase +
whitespace collapse) and silently changed **every** number — which is why the check is a
hard failure rather than a warning.

## What this closes, and what it does not

**Closes:** §5.5(4) is retired on this benchmark with a measured reason, not skipped.
Any future claim that go-cascade's cache is or is not subject to §2.9 must first report
an absorption rate; candidacy is not a proxy for it, and this benchmark is the
counterexample.

**Does not close:** §2.9 itself. The invariant (#8, calibrate on the cache-bypass
stream) is unchanged and still load-bearing — this measures that *this* benchmark cannot
exercise it, not that warm caches are harmless. Real traffic has genuine repeats;
independently-sampled coding-problem benchmarks are constructed to avoid them, which is
precisely why the absorption rate is ~0.

**The way to actually run §5.5(4)** is therefore not a bigger benchmark but a
**duplicate-injection** stream: replay a controlled fraction of queries (say 10–40%) so
absorption is a dial rather than a property of the corpus, then measure calibration
drift as a function of it. That is a benchmark-construction task, not a live-spend task,
and it is future work.

## Cost

**$0.** The approved ~$7 was not spent. The offline measurement took one process and no
credentials.
