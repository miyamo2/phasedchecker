package help

import (
	"flag"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/miyamo2/phasedchecker/internal/runner"
)

// Help writes help output to out based on args.
// When args is empty, it prints a summary of all phases and analyzers.
// When args[0] is "phase", it prints details for the named phase.
// When args[0] is "analyzer", it prints details for the named analyzer.
// For unrecognized topics, it calls log.Fatalf.
func Help(out io.Writer, progname string, pipeline runner.Pipeline, args []string) {
	if len(args) == 0 {
		printOverview(out, progname, pipeline)
		return
	}

	switch args[0] {
	case "phase":
		if len(args) < 2 {
			log.Fatalf("help phase: missing phase name")
		}
		printPhase(out, pipeline, args[1])

	case "analyzer":
		if len(args) < 2 {
			log.Fatalf("help analyzer: missing analyzer name")
		}
		printAnalyzer(out, pipeline, args[1])

	default:
		log.Fatalf("Unknown help topic %q. Use 'help', 'help phase <name>', or 'help analyzer <name>'.", args[0])
	}
}

// printOverview writes a summary of all phases and their analyzers.
func printOverview(out io.Writer, progname string, pipeline runner.Pipeline) {
	fmt.Fprintf(out, "%s is a tool for phase-based static analysis of Go programs.\n", progname)
	fmt.Fprintln(out)
	fmt.Fprintf(out, "%s runs analyzers in sequential phases — each phase completes\n", progname)
	fmt.Fprintln(out, "for ALL packages before the next phase starts.")
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Phases:")
	fmt.Fprintln(out)
	for _, phase := range pipeline.Phases {
		fmt.Fprintf(out, "  %s:\n", phase.Name)
		for _, a := range phase.Analyzers {
			title := strings.Split(a.Doc, "\n\n")[0]
			fmt.Fprintf(out, "    %-12s %s\n", a.Name, title)
		}
		fmt.Fprintln(out)
	}

	fmt.Fprintln(out, "Core flags:")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "  -fix         apply suggested fixes")
	fmt.Fprintln(out, "  -diff        with -fix, don't update the files, but print a unified diff")
	fmt.Fprintln(out, "  -json        emit JSON output to stdout")
	fmt.Fprintln(out, "  -test        indicates whether test files should be analyzed (default true)")
	fmt.Fprintf(out, "  -debug       debug flags, any subset of \"fpstv\"\n")
	fmt.Fprintln(out)

	fmt.Fprintf(out, "To see details of a specific phase, run '%s help phase <name>'.\n", progname)
	fmt.Fprintf(out, "To see details and flags of a specific analyzer, run '%s help analyzer <name>'.\n", progname)
}

// printPhase writes the details of the named phase.
func printPhase(out io.Writer, pipeline runner.Pipeline, name string) {
	for _, phase := range pipeline.Phases {
		if phase.Name == name {
			fmt.Fprintf(out, "Phase %q:\n", phase.Name)
			fmt.Fprintln(out)
			fmt.Fprintln(out, "  Analyzers:")
			fmt.Fprintln(out)
			for _, a := range phase.Analyzers {
				title := strings.Split(a.Doc, "\n\n")[0]
				fmt.Fprintf(out, "    %-12s %s\n", a.Name, title)
			}
			return
		}
	}
	log.Fatalf("Phase %q not found", name)
}

// printAnalyzer writes the details of the named analyzer.
func printAnalyzer(out io.Writer, pipeline runner.Pipeline, name string) {
	for _, phase := range pipeline.Phases {
		for _, a := range phase.Analyzers {
			if a.Name == name {
				paras := strings.Split(a.Doc, "\n\n")
				title := paras[0]
				fmt.Fprintf(out, "%s: %s\n", a.Name, title)

				// Show analyzer-specific flags if any exist.
				first := true
				fs := flag.NewFlagSet(a.Name, flag.ContinueOnError)
				a.Flags.VisitAll(func(f *flag.Flag) {
					if first {
						first = false
						fmt.Fprintln(out)
						fmt.Fprintln(out, "Analyzer flags:")
						fmt.Fprintln(out)
					}
					fs.Var(f.Value, a.Name+"."+f.Name, f.Usage)
				})
				if !first {
					fs.SetOutput(out)
					fs.PrintDefaults()
				}

				if len(paras) > 1 {
					fmt.Fprintf(out, "\n%s\n", strings.Join(paras[1:], "\n\n"))
				}
				return
			}
		}
	}
	log.Fatalf("Analyzer %q not found", name)
}
