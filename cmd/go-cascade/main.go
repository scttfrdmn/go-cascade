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
	"strings"
	"syscall"
	"time"

	"github.com/scttfrdmn/go-cascade/internal/cache"
	"github.com/scttfrdmn/go-cascade/internal/calibrate"
	"github.com/scttfrdmn/go-cascade/internal/cascade"
	"github.com/scttfrdmn/go-cascade/internal/config"
	"github.com/scttfrdmn/go-cascade/internal/model"
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
	if err := fs.Parse(args); err != nil {
		return err
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
		return nil
	}

	probs, err := readBench(*bench)
	if err != nil {
		return err
	}
	// Calibration records must come from the cache-bypass path.
	cfg.CacheDir = ""
	r, err := cascade.New(cfg, prov, nil)
	if err != nil {
		return err
	}
	defer r.Close() //nolint:errcheck // scratch dir

	recs := make([]calibrate.Record, 0, len(probs))
	for i, p := range probs {
		fmt.Fprintf(os.Stderr, "[%d/%d] %s\n", i+1, len(probs), p.ID)
		rec, err := r.Profile(ctx, p.ID, p.Problem)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  skipped: %v\n", err)
			continue
		}
		recs = append(recs, *rec)
	}
	if *recOut != "" {
		if err := writeJSONFile(*recOut, recs); err != nil {
			return err
		}
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
	return nil
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
