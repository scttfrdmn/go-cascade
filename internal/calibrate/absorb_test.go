package calibrate

import (
	"math"
	"testing"
)

// synthMix builds a stream whose tier-0 difficulty is *known*, so a pattern's
// selectivity can be checked against ground truth rather than eyeballed. Three
// classes, each present for a reason:
//
//	easy       tier-0 high score, correct        — what a warm cache absorbs
//	deceptive  tier-0 high score, WRONG          — makes serving cheap actually risky
//	hard       both tiers wrong, low tier-0 score — the irreducible risk floor
//
// Neither of the latter two is decoration; the fixture measured nothing without them.
//
// Without the deceptive class, tier-0 score separates correct from incorrect
// perfectly, so every interior threshold serves only correct candidates and there is
// no cost to serving cheap.
//
// Without a fallible final tier, the certificate simply escalates everything (tau_0 =
// 1) and risk is zero on *every* residual — so no absorption pattern can move it and
// a §2.9 test over such a stream passes while asserting nothing. That was this
// fixture's first version. The shift bites through the *composition* of the residual:
// absorbing the head raises the share of queries no tier gets right, so the same
// thresholds deliver more risk than they were calibrated for. A cascade whose last
// tier never errs has no risk to mis-certify, which is not the case the paper is
// about — the measured floor on the real n=409 records is 0.0587, not 0.
func synthMix(easy, deceptive, hard int) []Record {
	recs := make([]Record, 0, easy+deceptive+hard)
	tr := func(id string, score float64, cheap, frontier bool) Record {
		c, f := cheap, frontier
		return Record{
			ID: id,
			Tiers: []TierObs{
				{Tier: "small", Score: score, Correct: cheap, TrueCorrect: &c, Cost: 0.001},
				{Tier: "large", Score: 0.9, Correct: frontier, TrueCorrect: &f, Cost: 0.01},
			},
			Shadow: true, // as Profile writes them (invariant #8)
		}
	}
	for i := range easy {
		recs = append(recs, tr("easy"+itoa(i), 0.9, true, true))
	}
	for i := range deceptive {
		recs = append(recs, tr("dec"+itoa(i), 0.9, false, true))
	}
	for i := range hard {
		recs = append(recs, tr("hard"+itoa(i), 0.1, false, false))
	}
	return recs
}

// synthRecords is the two-class stream, for the tests that only care which records a
// pattern drops and not what the certificate does with them.
func synthRecords(easy, hard int) []Record { return synthMix(easy, 0, hard) }

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

func defaultOpts() AbsorptionOptions {
	return AbsorptionOptions{
		Tiers: []string{"small", "large"},
		LTT:   Options{Alpha: 0.10, Delta: 0.10, Step: 0.1},
		Seed:  42,
	}
}

// The central design claim of the harness: uniform absorption is a NULL CONTROL. It
// removes sample but not selectivity, so tier-0 accuracy on the residual must track
// the full stream. If this ever fails, the "uniform shows no effect" row stops being
// interpretable as a control and the whole sweep loses its baseline.
//
// This is why issue #52's naive framing — inject exact duplicates uniformly at
// random — would have measured nothing and reported it as evidence about §2.9.
func TestUniformAbsorptionDoesNotShiftDifficulty(t *testing.T) {
	recs := synthRecords(300, 100) // 75% easy
	base := tier0Accuracy(recs)
	for _, rho := range []float64{0.2, 0.4, 0.6, 0.8} {
		res, _, err := absorb(recs, AbsorbUniform, rho, defaultOpts())
		if err != nil {
			t.Fatal(err)
		}
		got := tier0Accuracy(res)
		// Tolerance is a sampling allowance, not slack: dropping 320 of 400 records
		// uniformly leaves 80, whose accuracy has a real standard error near 0.05.
		if math.Abs(got-base) > 0.10 {
			t.Errorf("rho=%.1f: uniform absorption moved tier-0 accuracy %.4f -> %.4f; "+
				"it is the null control and must not shift difficulty", rho, base, got)
		}
	}
}

// The mirror of the above, and the reason the harness reports anything at all: a
// head-shaped filter must actually shift difficulty, monotonically in rho. §2.9's
// premise is that the cache absorbs the recurring, easy queries; if easy-first did
// not move accuracy there would be no shift to correct and no experiment.
func TestSelectiveAbsorptionShiftsDifficultyMonotonically(t *testing.T) {
	recs := synthRecords(300, 100)
	base := tier0Accuracy(recs)
	prev := base
	for _, rho := range []float64{0.2, 0.4, 0.6, 0.75} {
		res, _, err := absorb(recs, AbsorbEasyFirst, rho, defaultOpts())
		if err != nil {
			t.Fatal(err)
		}
		got := tier0Accuracy(res)
		if got > prev+1e-9 {
			t.Errorf("rho=%.2f: tier-0 accuracy on the residual rose %.4f -> %.4f; "+
				"absorbing the easiest prefix can only make the residual harder", rho, prev, got)
		}
		prev = got
	}
	// At rho = 0.75 exactly the 300 easy records are gone, so the residual is all
	// hard. An exact figure, not a trend: it pins that the ordering really is by
	// difficulty and the drop count is not off by one.
	res, _, err := absorb(recs, AbsorbEasyFirst, 0.75, defaultOpts())
	if err != nil {
		t.Fatal(err)
	}
	if got := tier0Accuracy(res); got != 0 {
		t.Errorf("rho=0.75 of a 75%%-easy stream should leave only hard records; accuracy %.4f", got)
	}
	if len(res) != 100 {
		t.Errorf("residual has %d records, want the 100 hard ones", len(res))
	}
}

// The §2.9 failure itself, on a stream where the answer is known in advance:
// calibrate on the unshifted sample, deploy on a head-absorbed residual, and the
// certificate must come out optimistic — and more so as rho grows. A harness that
// reported a zero or negative gap here would be measuring nothing.
func TestUncorrectedCertificateIsOptimisticUnderSelectiveAbsorption(t *testing.T) {
	opts := defaultOpts()
	opts.Patterns = []AbsorptionPattern{AbsorbEasyFirst}
	opts.Rhos = []float64{0, 0.2, 0.4, 0.6}
	opts.Epsilons = []float64{0}

	// A 5% irreducible floor, under alpha = 0.10, so the unshifted stream really does
	// certify. That matters: the failure §2.9 describes is a certificate that is
	// *issued* and then betrayed by deployment, which is strictly worse than one that
	// was never issued. A fixture whose floor exceeds alpha would show the same gaps
	// with nothing ever certified, and would not demonstrate that.
	rows, err := SweepAbsorption(synthMix(320, 60, 20), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatalf("got %d rows, want 4", len(rows))
	}
	if !rows[0].Certified {
		t.Fatalf("the unshifted baseline must certify or there is no promise to break: %+v", rows[0])
	}
	if rows[0].RiskGap != 0 {
		t.Errorf("rho=0 must be a no-op: gap %+.4f", rows[0].RiskGap)
	}
	prev := rows[0].RiskGap
	for _, r := range rows[1:] {
		if r.RiskGap <= 0 {
			t.Errorf("rho=%.1f: gap %+.4f; calibrating on the unshifted stream and deploying "+
				"on a harder residual must understate risk", r.Rho, r.RiskGap)
		}
		if r.RiskGap < prev-1e-9 {
			t.Errorf("rho=%.1f: gap %+.4f is not larger than the previous %+.4f; the certificate "+
				"should grow more optimistic as the cache absorbs more of the head", r.Rho, r.RiskGap, prev)
		}
		prev = r.RiskGap
	}
	// And the promise is broken in the way that matters: the risk actually delivered
	// exceeds the alpha the certificate was issued at. A gap alone could sit entirely
	// under alpha and be a certificate that is merely loose rather than invalid.
	last := rows[len(rows)-1]
	if last.DeployedRisk <= opts.LTT.Alpha {
		t.Errorf("at rho=%.1f the deployed risk %.4f still respects alpha=%.2f; the sweep has not "+
			"exhibited an actually-violated certificate", last.Rho, last.DeployedRisk, opts.LTT.Alpha)
	}
}

// Shadow sampling's defining property: calibrate on a draw from the residual and the
// gap closes, because the calibration stream and the deployment stream are the same
// distribution. At epsilon = 1 they are the same *sample*, so the gap is exactly zero
// — the ideal-correction reference point the finite-epsilon rows are read against.
//
// Note what does NOT follow: closing the gap is not the same as certifying. Several
// eps=1 rows certify nothing, because the residual's true risk genuinely exceeds
// alpha. That is the correction working — refusing a bound it cannot support is the
// right behaviour, and is why Certified is reported alongside the gap.
func TestFullShadowSamplingClosesTheGapExactly(t *testing.T) {
	opts := defaultOpts()
	opts.Patterns = []AbsorptionPattern{AbsorbEasyFirst}
	opts.Rhos = []float64{0.2, 0.4, 0.6}
	opts.Epsilons = []float64{1.0}

	rows, err := SweepAbsorption(synthMix(240, 60, 100), opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.RiskGap != 0 {
			t.Errorf("rho=%.1f eps=1: gap %+.4f, want exactly 0 — calibrating on the whole "+
				"residual and deploying on it cannot disagree", r.Rho, r.RiskGap)
		}
		if r.NCalibration != r.NResidual {
			t.Errorf("rho=%.1f eps=1: calibrated on %d of %d residual records, want all",
				r.Rho, r.NCalibration, r.NResidual)
		}
	}
}

// The correction is not free: it buys distributional correctness by spending sample
// size. Below the (alpha, delta) floor a certificate cannot succeed even with zero
// errors, so such a row's gap is noise about sample size, not evidence about §2.9.
// It must be flagged — an unflagged row reads as a finding.
func TestUnderpoweredRowsAreFlaggedNotSilentlyReported(t *testing.T) {
	opts := defaultOpts()
	opts.Patterns = []AbsorptionPattern{AbsorbEasyFirst}
	opts.Rhos = []float64{0.5}
	opts.Epsilons = []float64{0.02, 1.0}

	rows, err := SweepAbsorption(synthMix(36, 24, 40), opts)
	if err != nil {
		t.Fatal(err)
	}
	floor := MinCalibrationSize(opts.LTT.Alpha, opts.LTT.Delta)
	for _, r := range rows {
		want := r.NCalibration < floor
		if r.Underpowered != want {
			t.Errorf("eps=%.2f n_cal=%d floor=%d: Underpowered=%v, want %v",
				r.Epsilon, r.NCalibration, floor, r.Underpowered, want)
		}
		if want && r.Note == "" {
			t.Errorf("eps=%.2f: an underpowered row carries no note, so it reads as a finding", r.Epsilon)
		}
	}
}

// The floor's closed form. At rhat = 0 the Hoeffding term is exactly (1-alpha)^n, so
// certification needs (1-alpha)^n <= delta. Check against direct evaluation of the
// p-value rather than against golden numbers: the point is that the floor is the same
// quantity HoeffdingBentkus enforces, not a heuristic that resembles it.
func TestMinCalibrationSizeIsWhereAZeroErrorBoundFirstSucceeds(t *testing.T) {
	for _, c := range []struct{ alpha, delta float64 }{
		{0.10, 0.10}, {0.05, 0.10}, {0.10, 0.05}, {0.20, 0.10}, {0.05, 0.01},
	} {
		n := MinCalibrationSize(c.alpha, c.delta)
		if n <= 0 {
			t.Fatalf("alpha=%v delta=%v: MinCalibrationSize returned %d", c.alpha, c.delta, n)
		}
		if p := HoeffdingBentkus(0, n, c.alpha); p > c.delta {
			t.Errorf("alpha=%v delta=%v: at the floor n=%d a zero-error bound still fails (p=%.4g > delta)",
				c.alpha, c.delta, n, p)
		}
		if n > 1 {
			if p := HoeffdingBentkus(0, n-1, c.alpha); p <= c.delta {
				t.Errorf("alpha=%v delta=%v: floor n=%d is not minimal — n-1 already certifies (p=%.4g)",
					c.alpha, c.delta, n, p)
			}
		}
	}
	// A degenerate configuration has no floor to report rather than a plausible one.
	for _, c := range []struct{ alpha, delta float64 }{{0, 0.1}, {1, 0.1}, {0.1, 0}, {0.1, 1}} {
		if n := MinCalibrationSize(c.alpha, c.delta); n != 0 {
			t.Errorf("alpha=%v delta=%v: want 0 for a degenerate config, got %d", c.alpha, c.delta, n)
		}
	}
}

// Two sweeps of the same records with the same seed must agree exactly. Without this
// a change in a row could be the dial or could be the shuffle, and no row would be
// attributable to rho — which is the only thing the sweep varies on purpose.
func TestSweepIsDeterministic(t *testing.T) {
	opts := defaultOpts()
	opts.Patterns = []AbsorptionPattern{AbsorbUniform, AbsorbEasyFirst, AbsorbCheapAccept}
	opts.Rhos = []float64{0.3, 0.6}
	opts.Epsilons = []float64{0, 0.3}
	opts.TauForCheapAccept = []float64{0.5, 0}

	recs := synthRecords(200, 100)
	a, err := SweepAbsorption(recs, opts)
	if err != nil {
		t.Fatal(err)
	}
	b, err := SweepAbsorption(recs, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != len(b) {
		t.Fatalf("row counts differ: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].RiskGap != b[i].RiskGap || a[i].NCalibration != b[i].NCalibration ||
			a[i].Certified != b[i].Certified || a[i].Tier0AccuracyResidual != b[i].Tier0AccuracyResidual {
			t.Errorf("row %d differs between identical sweeps:\n %+v\n %+v", i, a[i], b[i])
		}
	}
	// A different seed must actually change the uniform draw, or the seed is inert
	// and "deterministic" above would be vacuous.
	opts.Seed = 7
	c, err := SweepAbsorption(recs, opts)
	if err != nil {
		t.Fatal(err)
	}
	same := true
	for i := range a {
		if a[i].Tier0AccuracyResidual != c[i].Tier0AccuracyResidual {
			same = false
			break
		}
	}
	if same {
		t.Error("changing the seed changed no residual; the seed is not reaching the draw")
	}
}

// The sweep is offline analysis over borrowed records and must not disturb them. In
// particular Shadow is cleared on copies (see stripShadowFlags) — if that leaked back
// into the caller's slice, a later real Calibrate on the same records would silently
// stop preferring the shadow stream, which is invariant #8.
func TestSweepDoesNotMutateInput(t *testing.T) {
	recs := synthRecords(100, 50)
	before := make([]Record, len(recs))
	copy(before, recs)

	opts := defaultOpts()
	opts.Patterns = []AbsorptionPattern{AbsorbUniform, AbsorbEasyFirst}
	opts.Rhos = []float64{0.5}
	opts.Epsilons = []float64{0, 0.5}
	if _, err := SweepAbsorption(recs, opts); err != nil {
		t.Fatal(err)
	}
	for i := range recs {
		if recs[i].Shadow != before[i].Shadow || recs[i].ID != before[i].ID ||
			recs[i].Tiers[0].Score != before[i].Tiers[0].Score {
			t.Fatalf("record %d was mutated by the sweep: %+v -> %+v", i, before[i], recs[i])
		}
	}
}

// stripShadowFlags exists because Calibrate internally prefers the shadow subset of
// whatever it is handed, and every profiled record carries Shadow: true. Left set,
// that preference fires on BOTH arms and makes the uncorrected arm identical to the
// corrected one — which would read as "shadow sampling has no effect" and be an
// artifact of the harness rather than a result.
func TestStripShadowFlagsCopiesAndClears(t *testing.T) {
	in := synthRecords(3, 2)
	out := stripShadowFlags(in)
	for i := range out {
		if out[i].Shadow {
			t.Errorf("copy %d still has Shadow set", i)
		}
		if !in[i].Shadow {
			t.Errorf("input %d was cleared in place; the caller's records must be untouched", i)
		}
	}
}

// The dial has a domain. rho = 1 absorbs everything and leaves nothing to route, so
// it is excluded rather than clamped: a clamped 1.0 would silently report the rho
// nearest to it and the row would claim a rate it did not run.
func TestAbsorptionRateIsBoundsChecked(t *testing.T) {
	for _, rho := range []float64{-0.1, 1.0, 1.5} {
		if _, _, err := absorb(synthRecords(10, 10), AbsorbUniform, rho, defaultOpts()); err == nil {
			t.Errorf("rho=%v was accepted; the dial is defined on [0,1)", rho)
		}
	}
	if _, _, err := absorb(synthRecords(10, 10), "nonsense", 0.5, defaultOpts()); err == nil {
		t.Error("an unknown pattern was accepted; a typo would silently sweep the wrong thing")
	}
}

// rho = 0 is the identity, and the row it produces is the reference every other row
// is compared against. If it shifted anything the whole column of gaps would be
// measured against the wrong baseline.
func TestZeroAbsorptionIsTheIdentity(t *testing.T) {
	recs := synthRecords(50, 20)
	for _, pat := range []AbsorptionPattern{AbsorbUniform, AbsorbEasyFirst, AbsorbCheapAccept} {
		res, _, err := absorb(recs, pat, 0, defaultOpts())
		if err != nil {
			t.Fatal(err)
		}
		if len(res) != len(recs) {
			t.Errorf("%s at rho=0 dropped %d records", pat, len(recs)-len(res))
		}
		if got, want := tier0Accuracy(res), tier0Accuracy(recs); got != want {
			t.Errorf("%s at rho=0 changed accuracy %.4f -> %.4f", pat, want, got)
		}
	}
}

// cheap-accept absorbs what the *policy* served, so it depends on tau. Given a tau
// the cheap tier can clear it must absorb score-clearing records first; given none it
// must say so on the row rather than silently degrading to a different pattern.
func TestCheapAcceptDependsOnTauAndNotesItsAbsence(t *testing.T) {
	recs := synthRecords(200, 100)
	opts := defaultOpts()
	opts.TauForCheapAccept = []float64{0.5, 0}

	res, note, err := absorb(recs, AbsorbCheapAccept, 0.5, opts)
	if err != nil {
		t.Fatal(err)
	}
	if note != "" {
		t.Errorf("tau was supplied, so no fallback note is warranted; got %q", note)
	}
	// The 200 easy records score 0.9 and clear tau=0.5; dropping 150 removes only
	// those, so the residual is 50 easy + 100 hard.
	if got, want := tier0Accuracy(res), 50.0/150.0; math.Abs(got-want) > 1e-9 {
		t.Errorf("residual accuracy %.4f, want %.4f — the drop should take tau-clearing records first", got, want)
	}

	opts.TauForCheapAccept = nil
	if _, note, err = absorb(recs, AbsorbCheapAccept, 0.5, opts); err != nil {
		t.Fatal(err)
	}
	if note == "" {
		t.Error("cheap-accept with no tau fell back to score ordering without noting it")
	}
}

// Excluded records must not enter the sweep, for the same reason they do not enter a
// certificate: a contaminated or oracle-unsound record's labels are noise (invariants
// #3, #4). If they leaked in, a sweep row's n would not match what a real certificate
// over the same corpus would use, and the two would not be comparable.
func TestExcludedRecordsAreDroppedBeforeSweeping(t *testing.T) {
	recs := synthRecords(40, 20)
	recs[0].Contaminated = true
	recs[1].OracleUnsound = true
	// Inconclusive is explicitly NOT an exclusion — a naming mismatch is not an
	// unsound oracle, and Calibrate keeps those records.
	recs[2].OracleInconclusive = true

	if got, want := len(usableRecords(recs)), len(recs)-2; got != want {
		t.Errorf("usableRecords kept %d of %d, want %d", got, len(recs), want)
	}
	opts := defaultOpts()
	opts.Patterns = []AbsorptionPattern{AbsorbUniform}
	opts.Rhos = []float64{0}
	opts.Epsilons = []float64{0}
	rows, err := SweepAbsorption(recs, opts)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].NResidual != len(recs)-2 {
		t.Errorf("sweep at rho=0 saw %d records, want the %d usable ones", rows[0].NResidual, len(recs)-2)
	}
}

// A corpus with nothing usable is an error, not an empty sweep. An empty result would
// print a clean table of zero rows and read as "no effect found".
func TestSweepRejectsACorpusWithNoUsableRecords(t *testing.T) {
	recs := synthRecords(5, 5)
	for i := range recs {
		recs[i].Contaminated = true
	}
	if _, err := SweepAbsorption(recs, defaultOpts()); err == nil {
		t.Fatal("a fully-excluded corpus produced a sweep instead of an error")
	}
	if _, err := SweepAbsorption(synthRecords(5, 5), AbsorptionOptions{}); err == nil {
		t.Fatal("a sweep with no tier names succeeded; the grid arity comes from them")
	}
}

// The shadow draw must be independent of x — that is the property that makes it a
// draw from D rather than another selective filter. Check it behaviourally: the draw
// off a residual must preserve that residual's difficulty, at a size where it can.
func TestShadowDrawDoesNotSelectOnDifficulty(t *testing.T) {
	res := synthRecords(300, 100)
	base := tier0Accuracy(res)
	for _, eps := range []float64{0.25, 0.5, 0.75} {
		draw := shadowDraw(res, eps, 42, AbsorbEasyFirst, 0.5)
		if got, want := len(draw), int(math.Round(eps*float64(len(res)))); got != want {
			t.Errorf("eps=%.2f: drew %d records, want %d", eps, got, want)
		}
		if got := tier0Accuracy(draw); math.Abs(got-base) > 0.06 {
			t.Errorf("eps=%.2f: draw accuracy %.4f vs residual %.4f; the draw must not select on x",
				eps, got, base)
		}
	}
	// A draw can never be empty, or a row would report a certificate over nothing.
	if got := len(shadowDraw(res, 0.0001, 42, AbsorbUniform, 0)); got != 1 {
		t.Errorf("a vanishing epsilon drew %d records, want the 1-record minimum", got)
	}
}

// The asymmetry that makes the null envelope a per-seed quantity: ordering by difficulty
// is a sort, so the selective patterns are byte-identical across seeds, while the uniform
// draw is random and its risk gap moves. Consequence for anyone reading a sweep: a
// selective row's magnitude is exact, but the *control* it must beat has to be estimated
// over seeds. Quoting one sweep's max |gap| understates the envelope — measured at n=409,
// 0.0267 from a single seed against 0.0389 over ten, which is enough to flip the rho=0.4
// verdict. If this test ever fails because a selective pattern became seed-dependent, the
// published envelope has to be re-derived.
func TestSelectivePatternsAreSeedExactAndUniformIsNot(t *testing.T) {
	recs := synthMix(240, 60, 100)
	for _, pat := range []AbsorptionPattern{AbsorbEasyFirst, AbsorbCheapAccept} {
		var first []string
		for _, seed := range []uint64{1, 7, 99, 12345} {
			opts := defaultOpts()
			opts.Seed = seed
			opts.TauForCheapAccept = []float64{0.5, 0}
			res, _, err := absorb(recs, pat, 0.5, opts)
			if err != nil {
				t.Fatal(err)
			}
			ids := make([]string, len(res))
			for i, r := range res {
				ids[i] = r.ID
			}
			if first == nil {
				first = ids
				continue
			}
			if len(ids) != len(first) {
				t.Fatalf("%s: seed %d gave %d records, want %d", pat, seed, len(ids), len(first))
			}
			for i := range ids {
				if ids[i] != first[i] {
					t.Fatalf("%s: seed %d changed the residual at %d (%s vs %s); a difficulty sort "+
						"must not depend on the seed", pat, seed, i, ids[i], first[i])
				}
			}
		}
	}
	// Uniform must vary, or the null envelope would be a single number and there would
	// be nothing to estimate over seeds.
	seen := map[string]bool{}
	for _, seed := range []uint64{1, 7, 99, 12345} {
		opts := defaultOpts()
		opts.Seed = seed
		res, _, err := absorb(recs, AbsorbUniform, 0.5, opts)
		if err != nil {
			t.Fatal(err)
		}
		key := ""
		for _, r := range res {
			key += r.ID + ","
		}
		seen[key] = true
	}
	if len(seen) < 2 {
		t.Error("the uniform residual was identical across 4 seeds; the draw is not random, " +
			"so the null envelope cannot be estimated and the seed is inert")
	}
}

// The envelope must be a multi-seed quantity, and it must be at least as wide as any
// single sweep's — that is the whole point of computing it. A one-seed envelope is the
// bug this function exists to prevent, so the test asserts the estimate dominates a
// particular draw rather than merely that it is non-zero.
func TestNullEnvelopeIsWiderThanASingleSweep(t *testing.T) {
	// The hard class is the irreducible floor, so it has to sit *under* alpha or nothing
	// certifies, the envelope has zero draws, and the test asserts nothing.
	recs := synthMix(320, 60, 20)
	opts := defaultOpts()
	opts.Rhos = []float64{0, 0.2, 0.4, 0.6}
	opts.Epsilons = []float64{0}
	opts.Patterns = []AbsorptionPattern{AbsorbUniform}

	env, err := EstimateNullEnvelope(recs, opts, 8)
	if err != nil {
		t.Fatal(err)
	}
	if env == nil || env.NDraw == 0 {
		t.Fatal("no certified draws in the envelope; it would report a bar of 0 and every " +
			"selective row would clear it trivially")
	}
	if len(env.Seeds) != 8 {
		t.Errorf("envelope used %d seeds, want 8", len(env.Seeds))
	}
	// rho = 0 is the identity and contributes no sampling spread, so admitting it would
	// only drag the mean down and misstate how unusual the max is.
	for _, r := range env.Rhos {
		if r == 0 {
			t.Error("rho=0 entered the envelope; the identity transform is not a null draw")
		}
	}
	if env.MeanAbsGap > env.MaxAbsGap {
		t.Errorf("mean |gap| %.4f exceeds max %.4f", env.MeanAbsGap, env.MaxAbsGap)
	}

	// Any individual seed's sweep must be bounded by the estimate. If it is not, the
	// envelope is being computed over a different grid than it claims and a published
	// bar would be too narrow for the rows it is used to judge.
	for _, seed := range env.Seeds {
		one := opts
		one.Seed = seed
		one.Rhos = env.Rhos
		rows, err := SweepAbsorption(recs, one)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			if row.Underpowered || !row.Certified {
				continue
			}
			if g := math.Abs(row.RiskGap); g > env.MaxAbsGap+1e-12 {
				t.Errorf("seed %d rho %.2f gap %.4f exceeds the envelope %.4f it is supposed to bound",
					seed, row.Rho, g, env.MaxAbsGap)
			}
		}
	}
}

// Determinism, so a published envelope is reproducible from the command that printed
// it. The seeds are derived from opts.Seed rather than drawn, precisely so a reader can
// re-derive the number rather than take it on faith.
func TestNullEnvelopeIsDeterministicAndSeedDependent(t *testing.T) {
	recs := synthMix(320, 60, 20)
	opts := defaultOpts()
	opts.Rhos = []float64{0.2, 0.4, 0.6}
	opts.Epsilons = []float64{0}

	a, err := EstimateNullEnvelope(recs, opts, 6)
	if err != nil {
		t.Fatal(err)
	}
	b, err := EstimateNullEnvelope(recs, opts, 6)
	if err != nil {
		t.Fatal(err)
	}
	if a.MaxAbsGap != b.MaxAbsGap || a.MeanAbsGap != b.MeanAbsGap {
		t.Errorf("two runs at the same seed disagree: %v vs %v", a, b)
	}

	opts.Seed = 99
	c, err := EstimateNullEnvelope(recs, opts, 6)
	if err != nil {
		t.Fatal(err)
	}
	if c.Seeds[0] == a.Seeds[0] {
		t.Error("changing opts.Seed did not change the derived seeds; the envelope would be " +
			"the same 6 draws forever and its width could not be probed")
	}

	// Zero seeds is "skip", not "an envelope of 0" — a bar of zero would mark every
	// selective row significant, which is the opposite of the control's purpose.
	if env, err := EstimateNullEnvelope(recs, opts, 0); err != nil || env != nil {
		t.Errorf("nSeeds=0 returned (%v, %v), want (nil, nil) so callers can omit the bar", env, err)
	}
}
