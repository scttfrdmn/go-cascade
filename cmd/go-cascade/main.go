// Command go-cascade routes a coding problem to the cheapest model that
// provably solves it, verifying every candidate by execution.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/scttfrdmn/go-cascade/internal/cache"
	"github.com/scttfrdmn/go-cascade/internal/calibrate"
	"github.com/scttfrdmn/go-cascade/internal/cascade"
	"github.com/scttfrdmn/go-cascade/internal/config"
	"github.com/scttfrdmn/go-cascade/internal/model"
	"github.com/scttfrdmn/go-cascade/internal/prompt"
)

const usage = `go-cascade - route a Go coding problem to the cheapest model that provably solves it

Usage:
  go-cascade solve      [flags] "problem statement"
  go-cascade calibrate  [flags] -bench problems.jsonl
  go-cascade models     [flags]
  go-cascade cache      [stats|clear] [flags]

Run "go-cascade <command> -h" for the flags of each command.

Every candidate is executed against a test suite written, before any solution
exists, by a different model than the one that writes the code. A failed
verifier stage is a sound refutation, so the ladder can only reduce cost at
fixed risk. Thresholds carry a real guarantee only after "calibrate" has
produced a certificate; without one, runs are reported as UNCERTIFIED.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	switch os.Args[1] {
	case "solve":
		err = cmdSolve(ctx, os.Args[2:])
	case "calibrate":
		err = cmdCalibrate(ctx, os.Args[2:])
	case "models":
		err = cmdModels(ctx, os.Args[2:])
	case "cache":
		err = cmdCache(os.Args[2:])
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// commonFlags registers the knobs shared by solve and calibrate.
type commonFlags struct {
	cfgPath    string
	provider   string
	region     string
	alpha      float64
	budget     float64
	deadline   time.Duration
	thresholds string
	cacheDir   string
	noCache    bool
	mutants    int
	maxCx      int
	maxAllocs  int
	execWrap   string
	shadow     float64
}

func (f *commonFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&f.cfgPath, "config", "", "JSON config file layered over the defaults")
	fs.StringVar(&f.provider, "provider", "bedrock", "model provider: bedrock or mock")
	fs.StringVar(&f.region, "region", "", "AWS region (default from config, then us-west-2)")
	fs.Float64Var(&f.alpha, "alpha", 0, "target risk, e.g. 0.05; mutually exclusive with -budget")
	fs.Float64Var(&f.budget, "budget", 0, "USD ceiling per query; mutually exclusive with -alpha")
	fs.DurationVar(&f.deadline, "deadline", 0, "latency bound; switches to speculative parallel routing")
	fs.StringVar(&f.thresholds, "thresholds", "", "path to a calibration certificate")
	fs.StringVar(&f.cacheDir, "cache-dir", "", "cache location")
	fs.BoolVar(&f.noCache, "no-cache", false, "disable arm zero entirely")
	fs.IntVar(&f.mutants, "mutants", -1, "mutants for oracle-gap estimation; 0 disables")
	fs.IntVar(&f.maxCx, "max-complexity", 0, "reject solutions above this cyclomatic complexity")
	fs.IntVar(&f.maxAllocs, "max-allocs", 0, "reject solutions above this allocs/op (needs a benchmark)")
	fs.StringVar(&f.execWrap, "exec-wrapper", "", "command prefix used to sandbox test execution, e.g. 'firejail --net=none'")
	fs.Float64Var(&f.shadow, "shadow-rate", -1, "fraction of queries routed past the cache to keep calibration unbiased")
}

func (f *commonFlags) build() (*config.Config, model.Provider, *calibrate.Certificate, error) {
	cfg := config.Default()
	if f.cfgPath != "" {
		c, err := config.Load(f.cfgPath)
		if err != nil {
			return nil, nil, nil, err
		}
		cfg = c
	}
	if f.region != "" {
		cfg.Region = f.region
	}
	switch {
	case f.alpha > 0:
		cfg.Alpha, cfg.Budget = f.alpha, 0
	case f.budget > 0:
		cfg.Budget, cfg.Alpha = f.budget, 0
	}
	cfg.Deadline = f.deadline
	if f.cacheDir != "" {
		cfg.CacheDir = f.cacheDir
	}
	if f.noCache {
		cfg.CacheDir = ""
	}
	if f.mutants >= 0 {
		cfg.Mutants = f.mutants
	}
	cfg.MaxComplexity = f.maxCx
	cfg.MaxAllocsOp = f.maxAllocs
	if f.execWrap != "" {
		cfg.ExecWrapper = strings.Fields(f.execWrap)
	}
	if f.shadow >= 0 {
		cfg.ShadowRate = f.shadow
	}
	if f.thresholds != "" {
		cfg.ThresholdsPath = f.thresholds
	}

	var prov model.Provider
	switch f.provider {
	case "mock":
		prov = model.Mock{}
		// The mock has its own lineup; rewrite the tiers so the demo path does
		// not silently depend on Bedrock model IDs.
		ids := []string{model.MockSmall, model.MockMid, model.MockLarge}
		for i := range cfg.Tiers {
			if i < len(ids) {
				cfg.Tiers[i].ModelID = ids[i]
			}
		}
		cfg.TestModel = model.MockOracle
	case "bedrock":
		p, err := model.NewBedrock(context.Background(), cfg.Region)
		if err != nil {
			return nil, nil, nil, err
		}
		prov = p
	default:
		return nil, nil, nil, fmt.Errorf("unknown provider %q (want bedrock or mock)", f.provider)
	}

	cert, err := calibrate.Load(cfg.ThresholdsPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil, fmt.Errorf("load thresholds: %w", err)
	}
	return cfg, prov, cert, nil
}

func cmdSolve(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("solve", flag.ExitOnError)
	var cf commonFlags
	cf.register(fs)
	file := fs.String("f", "", "read the problem statement from a file ('-' for stdin)")
	out := fs.String("o", "", "write the accepted solution to this path")
	asJSON := fs.Bool("json", false, "emit the full result, including the trace, as JSON")
	quiet := fs.Bool("q", false, "print only the solution")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if cf.alpha == 0 && cf.budget == 0 {
		cf.alpha = 0.05 // a default is friendlier than an error; it is reported as such
	}

	problem, err := readProblem(*file, fs.Args())
	if err != nil {
		return err
	}

	cfg, prov, cert, err := cf.build()
	if err != nil {
		return err
	}
	r, err := cascade.New(cfg, prov, cert)
	if err != nil {
		return err
	}
	defer r.Close() //nolint:errcheck // scratch dir

	res, err := r.Solve(ctx, problem)
	if err != nil {
		return err
	}

	switch {
	case *asJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	case *quiet:
		if !res.Solved {
			return errors.New("no candidate survived the verifier ladder")
		}
		fmt.Println(res.Solution)
	default:
		printHuman(res)
	}
	if *out != "" && res.Solved {
		if err := os.WriteFile(*out, []byte(res.Solution+"\n"), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "\nwrote %s\n", *out)
	}
	if !res.Solved {
		os.Exit(3)
	}
	return nil
}

func printHuman(res *cascade.Result) {
	bold := func(s string) string { return "\033[1m" + s + "\033[0m" }
	fmt.Println(bold("route"))
	for _, s := range res.Trace {
		label := s.Stage
		if s.Tier != "" {
			label += "/" + s.Tier
		}
		fmt.Printf("  %-18s %-9s %-7s %s\n", label, s.Action,
			fmt.Sprintf("$%.5f", s.CostSoFar), s.Reason)
		if s.Diagnostic != "" {
			fmt.Printf("  %-18s %s\n", "", firstLine(s.Diagnostic))
		}
	}

	fmt.Println()
	fmt.Println(bold("outcome"))
	if !res.Solved {
		fmt.Println("  no candidate survived the verifier ladder")
	} else {
		fmt.Printf("  accepted at   %s (behavioural agreement %.2f)\n", res.AcceptedAt, res.Score)
	}
	fmt.Printf("  cost          $%.5f  (model $%.5f over %d calls, compute $%.5f over %s)\n",
		res.Cost.TotalUSD, res.Cost.ModelUSD, res.Cost.ModelCalls,
		res.Cost.ComputeUSD, res.Cost.VerifierTime.Round(time.Millisecond))
	fmt.Printf("  tokens        %d in / %d out\n", res.Cost.InputTokens, res.Cost.OutputTokens)
	fmt.Printf("  wall clock    %s\n", res.Elapsed.Round(time.Millisecond))
	fmt.Printf("  risk          %s\n", res.RiskStatement())
	if res.Static != nil {
		fmt.Printf("  static        complexity %d, %d funcs, imports %v\n",
			res.Static.MaxComplexity, res.Static.Funcs, res.Static.Imports)
	}
	if res.Solved {
		fmt.Println()
		fmt.Println(bold("solution"))
		fmt.Println(res.Solution)
	}
}

func cmdCalibrate(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("calibrate", flag.ExitOnError)
	var cf commonFlags
	cf.register(fs)
	bench := fs.String("bench", "", "JSONL file of {\"id\":..,\"problem\":..} calibration problems")
	delta := fs.Float64("delta", 0.1, "certificate confidence parameter")
	step := fs.Float64("step", 0.1, "threshold grid resolution")
	method := fs.String("method", string(calibrate.FixedSequence), "multiplicity control: fixed-sequence or bonferroni")
	outPath := fs.String("o", "thresholds.json", "certificate output path")
	recOut := fs.String("records", "", "also write the raw calibration records here")
	fromRec := fs.String("from-records", "", "replay calibration from saved records instead of profiling again")
	baselines := fs.Bool("baselines", false, "also report cascade vs always-cheapest vs always-frontier (cost and ground-truth risk)")
	oracle := fs.String("oracle", "execution", "acceptance oracle: execution (sound) or judge (LLM reviewer, §5.5c)")
	compare := fs.Bool("compare", false, "profile both oracles and report certified vs realized risk for each")
	judgeModel := fs.String("judge-model", "", "model that plays the judge oracle (default: test_model)")
	judgeStrict := fs.String("judge-strictness", "", "judge PASS/FAIL boundary on doubt: strict|balanced|permissive (default strict)")
	judgeSweep := fs.Bool("judge-sweep", false, "judge one shared candidate stream at every strictness level to trace the η_fa/β curve")
	judgeSeed := fs.Int("judge-seed", 0, "seeded dangerous-mode test: judge N known-wrong (killed-mutant) candidates per problem at every strictness level")
	seedKind := fs.String("seed-kind", "logic", "seeded-defect class: logic (single-edit mutants) or race (sync-deletion mutants, -race-refuted)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *oracle != "execution" && *oracle != "judge" {
		return fmt.Errorf("unknown -oracle %q (want execution or judge)", *oracle)
	}
	if *judgeStrict != "" && *judgeStrict != "strict" && *judgeStrict != "balanced" && *judgeStrict != "permissive" {
		return fmt.Errorf("unknown -judge-strictness %q (want strict, balanced, or permissive)", *judgeStrict)
	}
	if *bench == "" && *fromRec == "" {
		return errors.New("one of -bench or -from-records is required")
	}
	if cf.alpha == 0 {
		cf.alpha = 0.05
	}

	cfg, prov, _, err := cf.build()
	if err != nil {
		return err
	}
	names := make([]string, len(cfg.Tiers))
	for i, t := range cfg.Tiers {
		names[i] = t.Name
	}

	// Replaying saved records is the cheap path. Because every tier was
	// recorded on every problem, any threshold vector -- and any alpha -- can
	// be re-evaluated offline without querying a model again.
	if *fromRec != "" {
		var recs []calibrate.Record
		b, err := os.ReadFile(*fromRec)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(b, &recs); err != nil {
			return fmt.Errorf("parse %s: %w", *fromRec, err)
		}
		cert, err := calibrate.Calibrate(recs, names, calibrate.Options{
			Alpha: cfg.Alpha, Delta: *delta, Step: *step, Method: calibrate.Method(*method),
		})
		if err != nil {
			return err
		}
		if err := calibrate.Save(*outPath, cert); err != nil {
			return err
		}
		printCert(*outPath, cert)
		// The cascade only earns its keep if it beats the single-model policies:
		// cheaper than always-frontier at no worse correctness, more correct than
		// always-cheapest. Score all three on the same records, under the
		// certified thresholds, on ground truth.
		if *baselines {
			printBaselines(calibrate.Baselines(recs, cert.Thresholds))
		}
		return nil
	}

	probs, err := readBench(*bench)
	if err != nil {
		return err
	}
	// Calibration records must come from the cache-bypass path.
	cfg.CacheDir = ""
	cfg.JudgeStrictness = *judgeStrict

	opts := calibrate.Options{
		Alpha: cfg.Alpha, Delta: *delta, Step: *step, Method: calibrate.Method(*method),
	}

	// Seeded dangerous-mode test: judge N provably-wrong candidates per problem
	// at every strictness level. Unlike the sweeps, this does not depend on the
	// models emitting a wrong candidate — wrong candidates are seeded, so η_fa is
	// directly measurable.
	if *judgeSeed > 0 {
		if *seedKind != "logic" && *seedKind != "race" {
			return fmt.Errorf("unknown -seed-kind %q (want logic or race)", *seedKind)
		}
		kind := cascade.SeedLogic
		if *seedKind == "race" {
			kind = cascade.SeedRace
		}
		return runSeededSweep(ctx, cfg, prov, probs, *judgeModel, *judgeSeed, kind)
	}

	// Judge-sweep runs --compare once per strictness level to trace the judge
	// oracle's η_fa/β operating curve. Only the judge arm's prompt changes; the
	// execution arm is identical across levels, so it is reported once.
	if *judgeSweep {
		return runJudgeSweep(ctx, cfg, prov, probs, names, opts, *judgeModel, *recOut)
	}

	r, err := cascade.New(cfg, prov, nil)
	if err != nil {
		return err
	}
	defer r.Close() //nolint:errcheck // scratch dir

	// Compare mode profiles both oracles on the same problems and prints the
	// certified-vs-realized risk for each. This is the §5.5c experiment in
	// miniature: the execution arm's realized risk should match its certified
	// alpha, while the judge arm's realized risk can exceed it.
	if *compare {
		return runCompare(ctx, r, probs, names, opts, *judgeModel, *recOut)
	}

	recs := make([]calibrate.Record, 0, len(probs))
	for i, p := range probs {
		fmt.Fprintf(os.Stderr, "[%d/%d] %s (oracle=%s)\n", i+1, len(probs), p.ID, *oracle)
		rec, perr := profileOne(ctx, r, p, *oracle, *judgeModel)
		if perr != nil {
			fmt.Fprintf(os.Stderr, "  skipped: %v\n", perr)
			continue
		}
		recs = append(recs, *rec)
	}
	if *recOut != "" {
		if err := writeJSONFile(*recOut, recs); err != nil {
			return err
		}
	}

	cert, err := calibrate.Calibrate(recs, names, opts)
	if err != nil {
		return err
	}
	if err := calibrate.Save(*outPath, cert); err != nil {
		return err
	}

	printCert(*outPath, cert)
	return nil
}

// profileOne dispatches to the execution or judge oracle. The execution oracle
// (Profile) is sound; the judge oracle (ProfileJudge) records execution truth
// alongside the judge verdict so the certificate can be checked against reality.
func profileOne(ctx context.Context, r *cascade.Router, p benchProblem, oracle, judgeModel string) (*calibrate.Record, error) {
	if oracle == "judge" {
		return r.ProfileJudge(ctx, p.ID, p.Problem, judgeModel)
	}
	return r.Profile(ctx, p.ID, p.Problem)
}

// armRecordPath inserts the arm name before the extension of the -records path,
// e.g. ("records.json", "judge") -> "records.judge.json", so the two arms do
// not overwrite each other.
func armRecordPath(recOut, arm string) string {
	ext := filepath.Ext(recOut)
	stem := strings.TrimSuffix(recOut, ext)
	if ext == "" {
		ext = ".json"
	}
	return stem + "." + arm + ext
}

// runSeededSweep runs the seeded dangerous-mode test: for each problem it
// harvests provably-wrong candidates (killed mutants) and judges each at every
// strictness level, then reports the false-acceptance rate per level. Because
// every judged candidate is known-wrong by execution, any PASS is an
// unambiguous η_fa, and a rate that climbs as the judge loosens is the §3.1
// danger demonstrated directly rather than left to chance.
func runSeededSweep(ctx context.Context, cfg *config.Config, prov model.Provider, probs []benchProblem, judgeModel string, nSeed int, seedKind cascade.SeedKind) error {
	levels := []prompt.JudgeStrictness{prompt.JudgeStrict, prompt.JudgeBalanced, prompt.JudgePermissive}
	r, err := cascade.New(cfg, prov, nil)
	if err != nil {
		return err
	}
	defer r.Close() //nolint:errcheck // scratch dir

	totals := map[prompt.JudgeStrictness]*cascade.SeededJudgeResult{}
	for _, lvl := range levels {
		totals[lvl] = &cascade.SeededJudgeResult{Level: lvl}
	}
	usedProblems, totalWrong := 0, 0
	for i, p := range probs {
		fmt.Fprintf(os.Stderr, "[seed %d/%d] %s\n", i+1, len(probs), p.ID)
		res, nWrong, perr := r.ProfileSeeded(ctx, p.Problem, judgeModel, nSeed, levels, seedKind)
		if perr != nil {
			fmt.Fprintf(os.Stderr, "  skipped: %v\n", perr)
			continue
		}
		if res == nil || nWrong == 0 {
			fmt.Fprintf(os.Stderr, "  no provably-wrong candidate; skipped\n")
			continue
		}
		usedProblems++
		totalWrong += nWrong
		for _, lvl := range levels {
			totals[lvl].Judged += res[lvl].Judged
			totals[lvl].FalseAccept += res[lvl].FalseAccept
		}
	}

	fmt.Printf("\nSeeded dangerous-mode test: %d known-wrong candidates from %d problems, judged at each level.\n",
		totalWrong, usedProblems)
	fmt.Printf("\n%-11s %-8s %-12s %s\n", "strictness", "judged", "false-acc", "η_fa rate")
	fmt.Println("  " + strings.Repeat("-", 44))
	for _, lvl := range levels {
		t := totals[lvl]
		rate := 0.0
		if t.Judged > 0 {
			rate = float64(t.FalseAccept) / float64(t.Judged)
		}
		fmt.Printf("%-11s %-8d %-12d %.3f\n", lvl, t.Judged, t.FalseAccept, rate)
	}
	fmt.Println("\nEvery candidate is provably wrong (execution refutes it), so every PASS is a")
	fmt.Println("false acceptance. If the η_fa rate rises as the judge loosens, the §3.1")
	fmt.Println("dangerous mode is directly demonstrated: a permissive judge certifies code")
	fmt.Println("the tests reject.")
	return nil
}

// runJudgeSweep traces the judge oracle's false-acceptance (η_fa) /
// false-rejection (β) operating curve across strictness levels, judging the SAME
// candidate stream at every level. The candidates are sampled once per problem
// and execution truth is fixed; only the judge's PASS/FAIL tie-break changes. A
// verdict that flips as the judge loosens is therefore attributable to
// strictness alone — the controlled A/B the earlier re-sampling sweep could not
// provide. The point is to test whether the §3.1 dangerous mode (a judge
// certifying below its true risk) is reachable by loosening the judge.
func runJudgeSweep(ctx context.Context, cfg *config.Config, prov model.Provider, probs []benchProblem, names []string, opts calibrate.Options, judgeModel, recOut string) error {
	levels := []prompt.JudgeStrictness{prompt.JudgeStrict, prompt.JudgeBalanced, prompt.JudgePermissive}
	r, err := cascade.New(cfg, prov, nil)
	if err != nil {
		return err
	}
	defer r.Close() //nolint:errcheck // scratch dir

	var execRecs []calibrate.Record
	judgeRecs := map[prompt.JudgeStrictness][]calibrate.Record{}
	for i, p := range probs {
		fmt.Fprintf(os.Stderr, "[sweep %d/%d] %s\n", i+1, len(probs), p.ID)
		er, jrs, perr := r.ProfileStrictnessReplay(ctx, p.ID, p.Problem, judgeModel, levels)
		if perr != nil {
			fmt.Fprintf(os.Stderr, "  skipped: %v\n", perr)
			continue
		}
		execRecs = append(execRecs, *er)
		for _, lvl := range levels {
			judgeRecs[lvl] = append(judgeRecs[lvl], *jrs[lvl])
		}
	}

	if recOut != "" {
		if err := writeJSONFile(armRecordPath(recOut, "execution"), execRecs); err != nil {
			return fmt.Errorf("write execution records: %w", err)
		}
		for _, lvl := range levels {
			if err := writeJSONFile(armRecordPath(recOut, string(lvl)), judgeRecs[lvl]); err != nil {
				return fmt.Errorf("write %s records: %w", lvl, err)
			}
		}
	}

	// The execution arm is identical across levels (strictness-independent), so
	// report it once as the ground-truth baseline.
	execCert, err := calibrate.Calibrate(execRecs, names, opts)
	if err != nil {
		return fmt.Errorf("execution arm: %w", err)
	}
	fmt.Printf("\nexecution baseline (all levels): risk %.4f over n=%d\n", execCert.EmpiricalRisk, execCert.N)

	fmt.Printf("\n%-11s %-9s %-9s %-6s %-6s\n", "strictness", "judge-emp", "judge-real", "η_fa", "β")
	fmt.Println("  " + strings.Repeat("-", 50))
	for _, lvl := range levels {
		var fa, fr int
		for i := range execRecs {
			f, b := countJudgeErrors(execRecs[i], judgeRecs[lvl][i])
			fa += f
			fr += b
		}
		cert, err := calibrate.Calibrate(judgeRecs[lvl], names, opts)
		if err != nil {
			return fmt.Errorf("%s arm: %w", lvl, err)
		}
		fmt.Printf("%-11s %-9.4f %-9.4f %-6d %-6d\n",
			lvl, cert.EmpiricalRisk, cert.RealizedRisk, fa, fr)
	}
	fmt.Println("\nSame candidates at every level, so any η_fa/β movement is the strictness")
	fmt.Println("knob alone. η_fa = judge passed a program the tests refute (the dangerous")
	fmt.Println("mode). β = judge failed a program the tests accept. Rising η_fa as the judge")
	fmt.Println("loosens means the §3.1 danger is reachable by prompt.")
	return nil
}

// countJudgeErrors tallies, across all tiers of one paired record, how often the
// judge's verdict disagreed with execution truth: false acceptances (passed a
// wrong program) and false rejections (failed a correct one).
func countJudgeErrors(execRec, judgeRec calibrate.Record) (falseAccept, falseReject int) {
	for i := range execRec.Tiers {
		if i >= len(judgeRec.Tiers) {
			break
		}
		truth := execRec.Tiers[i].TrueCorrect
		if truth == nil {
			continue
		}
		jc := judgeRec.Tiers[i].Correct
		switch {
		case jc && !*truth:
			falseAccept++
		case !jc && *truth:
			falseReject++
		}
	}
	return falseAccept, falseReject
}

// runCompare profiles both oracles against a single shared candidate stream and
// prints a side-by-side of what each certifies against what it actually
// delivers. Both arms rule on the identical spec and candidates per problem, so
// any difference between them is attributable to the oracle rather than to
// independent sampling variance. When recOut is set, each arm's raw records are
// written to <recOut-stem>.<arm>.json so a live run can be replayed offline at
// any alpha rather than thrown away.
func runCompare(ctx context.Context, r *cascade.Router, probs []benchProblem, names []string, opts calibrate.Options, judgeModel, recOut string) error {
	execRecs := make([]calibrate.Record, 0, len(probs))
	judgeRecs := make([]calibrate.Record, 0, len(probs))
	for i, p := range probs {
		fmt.Fprintf(os.Stderr, "[paired %d/%d] %s\n", i+1, len(probs), p.ID)
		er, jr, err := r.ProfilePaired(ctx, p.ID, p.Problem, judgeModel)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  skipped: %v\n", err)
			continue
		}
		execRecs = append(execRecs, *er)
		judgeRecs = append(judgeRecs, *jr)
	}

	if recOut != "" {
		for name, recs := range map[string][]calibrate.Record{"execution": execRecs, "judge": judgeRecs} {
			path := armRecordPath(recOut, name)
			if err := writeJSONFile(path, recs); err != nil {
				return fmt.Errorf("write %s records: %w", name, err)
			}
			fmt.Fprintf(os.Stderr, "wrote %d %s records to %s\n", len(recs), name, path)
		}
	}

	fmt.Printf("\n%-11s %-6s %-9s %-9s %-9s %s\n",
		"arm", "valid", "cert-α", "emp-risk", "real-risk", "verdict")
	fmt.Println("  " + strings.Repeat("-", 68))
	for _, a := range []struct {
		name string
		recs []calibrate.Record
	}{{"execution", execRecs}, {"judge", judgeRecs}} {
		cert, err := calibrate.Calibrate(a.recs, names, opts)
		if err != nil {
			return fmt.Errorf("%s arm: %w", a.name, err)
		}
		verdict := "certified risk holds"
		switch {
		case !cert.Valid:
			verdict = "could not certify"
		case cert.RealizedRisk > cert.Alpha+1e-9:
			verdict = "NOMINAL ONLY: realized risk exceeds α (judge noise floor)"
		}
		fmt.Printf("%-11s %-6v %-9.3f %-9.4f %-9.4f %s\n",
			a.name, cert.Valid, cert.Alpha, cert.EmpiricalRisk, cert.RealizedRisk, verdict)
	}
	fmt.Println("\nBoth arms ruled on the same candidates, so the difference is the oracle,")
	fmt.Println("not sampling. The execution oracle is sound (β=0): realized risk equals")
	fmt.Println("empirical. Where the judge's realized risk exceeds its empirical risk, that")
	fmt.Println("gap is the false-acceptance rate it certified against but cannot see.")
	return nil
}

// printBaselines renders the cascade against the two single-model policies it
// must beat. The comparison is on ground-truth correctness and measured cost, so
// it answers "is the cascade actually the better choice", not "which risk can it
// certify".
func printBaselines(ps []calibrate.Policy) {
	fmt.Printf("\nbaselines (ground-truth risk, mean cost/query, n)\n")
	fmt.Printf("  %-18s %-10s %-12s %s\n", "policy", "risk", "mean-cost", "n")
	fmt.Println("  " + strings.Repeat("-", 48))
	for _, p := range ps {
		fmt.Printf("  %-18s %-10.4f $%-11.5f %d\n", p.Name, p.Risk, p.MeanUSD, p.N)
	}
	fmt.Println("\nThe cascade earns its keep only if it is cheaper than always-frontier at no")
	fmt.Println("worse risk, and lower-risk than always-cheapest. Compare the rows above.")
}

func printCert(path string, cert *calibrate.Certificate) {
	fmt.Printf("\ncertificate: %s\n", path)
	fmt.Printf("  valid           %v\n", cert.Valid)
	fmt.Printf("  alpha / delta   %.3f / %.3f\n", cert.Alpha, cert.Delta)
	fmt.Printf("  n               %d (%d excluded as contaminated, %d shadow)\n",
		cert.N, cert.NExcluded, cert.NShadow)
	fmt.Printf("  method          %s over a grid of %d\n", cert.Method, cert.GridSize)
	fmt.Printf("  thresholds      %v\n", cert.Thresholds)
	fmt.Printf("  empirical risk  %.4f (p=%.4g)\n", cert.EmpiricalRisk, cert.PValue)
	fmt.Printf("  realized risk   %.4f (ground truth)\n", cert.RealizedRisk)
	fmt.Printf("  expected cost   $%.5f per query\n", cert.ExpectedCost)
	if cert.Note != "" {
		fmt.Printf("  note            %s\n", cert.Note)
	}
}

func cmdModels(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("models", flag.ExitOnError)
	region := fs.String("region", "us-west-2", "AWS region")
	filter := fs.String("filter", "", "substring filter on the profile ID")
	if err := fs.Parse(args); err != nil {
		return err
	}
	b, err := model.NewBedrock(ctx, *region)
	if err != nil {
		return err
	}
	profiles, err := b.ListProfiles(ctx)
	if err != nil {
		return err
	}
	n := 0
	for _, p := range profiles {
		if *filter != "" && !strings.Contains(p.ID, *filter) {
			continue
		}
		fmt.Printf("%-60s %-10s %s\n", p.ID, p.Status, p.Name)
		n++
	}
	if n == 0 {
		fmt.Fprintln(os.Stderr, "no inference profiles matched; check the region and model access")
	}
	return nil
}

func cmdCache(args []string) error {
	fs := flag.NewFlagSet("cache", flag.ExitOnError)
	dir := fs.String("cache-dir", config.Default().CacheDir, "cache location")
	sub := "stats"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		sub, args = args[0], args[1:]
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	switch sub {
	case "stats":
		c, err := cache.Open(*dir)
		if err != nil {
			return err
		}
		s := c.Stats()
		fmt.Printf("%s\n  solutions %d\n  specs     %d\n  failures  %d\n",
			*dir, s.Solutions, s.Specs, s.Failures)
	case "clear":
		if err := os.RemoveAll(*dir); err != nil {
			return err
		}
		fmt.Printf("removed %s\n", *dir)
	default:
		return fmt.Errorf("unknown cache subcommand %q", sub)
	}
	return nil
}

type benchProblem struct {
	ID      string `json:"id"`
	Problem string `json:"problem"`
}

func readBench(path string) ([]benchProblem, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // read-only

	var out []benchProblem
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for line := 1; sc.Scan(); line++ {
		t := strings.TrimSpace(sc.Text())
		if t == "" || strings.HasPrefix(t, "//") {
			continue
		}
		var p benchProblem
		if err := json.Unmarshal([]byte(t), &p); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, line, err)
		}
		if p.ID == "" {
			p.ID = fmt.Sprintf("p%03d", line)
		}
		out = append(out, p)
	}
	return out, sc.Err()
}

func readProblem(file string, rest []string) (string, error) {
	switch {
	case file == "-":
		b, err := os.ReadFile("/dev/stdin")
		return string(b), err
	case file != "":
		b, err := os.ReadFile(file)
		return string(b), err
	case len(rest) > 0:
		return strings.Join(rest, " "), nil
	}
	return "", errors.New("no problem statement: pass it as an argument or with -f")
}

func writeJSONFile(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 160 {
		s = s[:160] + "..."
	}
	return s
}
