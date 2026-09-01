package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	goooruntime "github.com/kimjooyoon/gooo-two-generation-bootstrap/runtime"
)

func main() {
	if len(os.Args) < 2 {
		fail(errors.New("command is required: generate, compare, or conformance"))
	}
	switch os.Args[1] {
	case "generate":
		generate(os.Args[2:])
	case "compare":
		compare(os.Args[2:])
	case "conformance":
		conformance(os.Args[2:])
	default:
		fail(fmt.Errorf("unknown command %q", os.Args[1]))
	}
}

func generate(args []string) {
	flags := flag.NewFlagSet("generate", flag.ContinueOnError)
	meta := flags.String("meta", ".gooo/two-generation.gooo", "authoritative .gooo metacode")
	input := flags.String("input", "", "same .gooo input for stage0")
	output := flags.String("output-dir", "", "caller-owned temporary output directory")
	repoRoot := flags.String("repo-root", ".", "target repository root")
	if err := flags.Parse(args); err != nil {
		fail(err)
	}
	if err := goooruntime.ExecuteStage0(*meta, *input, *output, *repoRoot); err != nil {
		fail(err)
	}
}

func compare(args []string) {
	flags := flag.NewFlagSet("compare", flag.ContinueOnError)
	metaPath := flags.String("meta", ".gooo/two-generation.gooo", "authoritative .gooo metacode")
	stage1 := flags.String("stage1-dir", "", "stage1 output directory")
	stage2 := flags.String("stage2-dir", "", "stage2 output directory")
	output := flags.String("output", "", "caller-owned fixed-point report")
	if err := flags.Parse(args); err != nil {
		fail(err)
	}
	meta, _, err := goooruntime.LoadMeta(*metaPath)
	if err != nil {
		fail(err)
	}
	report := goooruntime.Compare(*stage1, *stage2, meta)
	if err := goooruntime.WriteJSON(*output, report); err != nil {
		fail(err)
	}
	if report.Status != goooruntime.StatusClosed {
		fail(fmt.Errorf("fixed-point status is %s: %s", report.Status, report.Reason))
	}
}

func conformance(args []string) {
	flags := flag.NewFlagSet("conformance", flag.ContinueOnError)
	metaPath := flags.String("meta", ".gooo/two-generation.gooo", "authoritative .gooo metacode")
	repoRoot := flags.String("repo-root", ".", "target repository root")
	output := flags.String("output", "", "caller-owned conformance report")
	if err := flags.Parse(args); err != nil {
		fail(err)
	}
	meta, raw, err := goooruntime.LoadMeta(*metaPath)
	if err != nil {
		fail(err)
	}
	report, err := goooruntime.RunCorpus(meta, raw, *repoRoot)
	if err != nil {
		fail(err)
	}
	if err := goooruntime.WriteJSON(*output, report); err != nil {
		fail(err)
	}
	if report.Status != goooruntime.StatusClosed {
		fail(fmt.Errorf("corpus status is %s", report.Status))
	}
}

func fail(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(1)
}
