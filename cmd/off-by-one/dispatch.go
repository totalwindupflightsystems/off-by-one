// Package main — CLI subcommand dispatch.
//
// The stdlib flag package stops parsing at the first non-flag argument, so
// `off-by-one --db PATH seed` leaves `seed` in flag.Args() and boots the HTTP
// server instead of seeding (OB-GAP-071). seedDispatch walks argv with a probe
// flag set built from the live server declarations — same names, same arity,
// values discarded — to find the first positional argument without guessing
// which flags take values.
package main

import (
	"flag"
	"io"
	"log"
	"strings"
)

// seedSubcommand is the one subcommand recognised before flag.Parse: it owns
// its own flags and must never reach the server startup path.
const seedSubcommand = "seed"

// dispatchValue is a throwaway flag.Value: the probe set registers one per
// server flag so the probe consumes exactly the arguments the real set would.
// The value is recorded (the seed subcommand shares -db with the server) and
// otherwise ignored.
type dispatchValue struct {
	text   string
	set    bool
	isBool bool
}

func (v *dispatchValue) String() string { return v.text }

func (v *dispatchValue) Set(s string) error {
	v.text, v.set = s, true
	return nil
}

// IsBoolFlag mirrors the flag package's own arity test (an interface check on
// the Value, not on the flag's default): a boolean flag may be given without a
// value, so `-readonly seed` must not swallow the subcommand.
func (v *dispatchValue) IsBoolFlag() bool { return v.isBool }

// newDispatchProbe returns a flag set mirroring server's flag names and arity.
// Output is discarded and parsing never exits — the probe is a recogniser, and
// every real diagnostic stays with flag.Parse on the server path.
func newDispatchProbe(server *flag.FlagSet) *flag.FlagSet {
	probe := flag.NewFlagSet("off-by-one", flag.ContinueOnError)
	probe.SetOutput(io.Discard)
	probe.Usage = func() {}
	if server == nil {
		return probe
	}
	server.VisitAll(func(f *flag.Flag) {
		v := &dispatchValue{}
		if bf, ok := f.Value.(interface{ IsBoolFlag() bool }); ok {
			v.isBool = bf.IsBoolFlag()
		}
		probe.Var(v, f.Name, "")
	})
	return probe
}

// explicitValue returns the value recorded for name and whether the flag was
// actually given (Visit only reports explicit flags, but the probe value also
// carries the raw text).
func explicitValue(probe *flag.FlagSet, name string) (string, bool) {
	f := probe.Lookup(name)
	if f == nil {
		return "", false
	}
	v, ok := f.Value.(*dispatchValue)
	if !ok || !v.set {
		return "", false
	}
	return v.text, true
}

// seedDispatch reports whether argv (os.Args[1:]) selects the `seed`
// subcommand and returns the argument list to hand to seedRun.
//
// All of these select it:
//
//	off-by-one seed
//	off-by-one seed -dir DIR -db DB          (historical form, unchanged)
//	off-by-one --db DB seed
//	off-by-one --db=DB --port 9000 seed
//	off-by-one --readonly seed               (boolean flags do not eat "seed")
//
// Flags given before `seed` are not silently reinterpreted: `-db` is forwarded
// in its seed spelling, and any other explicit server flag is named on stderr
// as ignored, so `off-by-one --port 9000 seed` cannot look like it seeded into
// :9000. A `-db` after the subcommand still wins, because the seed FlagSet
// applies occurrences in order.
//
// The server path is preserved for everything else. A probe parse error
// (--help/-h, an unknown flag, a missing flag value) and an explicit --version
// are NOT seed: argv falls through to flag.Parse, which prints the normal
// usage/version output and exits exactly as before.
func seedDispatch(argv []string, server *flag.FlagSet) ([]string, bool) {
	probe := newDispatchProbe(server)
	if err := probe.Parse(argv); err != nil {
		// ErrHelp, "flag provided but not defined", "flag needs an argument":
		// the server path owns these diagnostics.
		return nil, false
	}

	rest := probe.Args()
	if len(rest) == 0 || rest[0] != seedSubcommand {
		// No positional argument, or a different one: `--db seed` names a
		// database file, `--port 9000` is an ordinary server start.
		return nil, false
	}

	// `off-by-one --version seed`: the server flag set owns --version and
	// printing it keeps the pre-subcommand-aware precedence.
	if _, ok := explicitValue(probe, "version"); ok {
		return nil, false
	}

	seedArgs := make([]string, 0, len(rest)+2)
	var ignored []string
	probe.Visit(func(f *flag.Flag) {
		if f.Name == "db" {
			if val, ok := explicitValue(probe, "db"); ok {
				seedArgs = append(seedArgs, "-db", val)
			}
			return
		}
		ignored = append(ignored, "-"+f.Name)
	})
	if len(ignored) > 0 {
		log.Printf("seed: ignoring server flag(s) given before the subcommand: %s", strings.Join(ignored, ", "))
	}

	return append(seedArgs, rest[1:]...), true
}
