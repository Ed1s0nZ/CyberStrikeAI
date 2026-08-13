package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	"cyberstrike-ai/internal/promptaudit"
)

func main() {
	root := flag.String("root", ".", "repository root")
	format := flag.String("format", "text", "output format: text or json")
	maxFindings := flag.Int("max-findings", 50, "maximum findings printed in text mode; 0 prints all")
	flag.Parse()

	report, err := promptaudit.Audit(*root, promptaudit.DefaultLimits())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		return
	}
	if *format != "text" {
		fmt.Fprintf(os.Stderr, "unsupported format %q\n", *format)
		os.Exit(2)
	}

	s := report.Summary
	fmt.Printf("agents=%d roles=%d skills=%d invalid=%d/%d/%d oversized=%d/%d/%d broken_references=%d internal_name_mismatch=%d\n",
		s.Agents, s.Roles, s.Skills, s.InvalidAgents, s.InvalidRoles, s.InvalidSkills,
		s.OversizedAgents, s.OversizedRoles, s.OversizedSkills, s.BrokenReferences, s.InternalNameMismatch)
	keys := make([]string, 0, len(s.MissingSections))
	for key := range s.MissingSections {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	fmt.Print("missing_skill_sections")
	for _, key := range keys {
		fmt.Printf(" %s=%d", key, s.MissingSections[key])
	}
	fmt.Println()

	limit := len(report.Findings)
	if *maxFindings > 0 && limit > *maxFindings {
		limit = *maxFindings
	}
	for _, finding := range report.Findings[:limit] {
		fmt.Printf("[%s] %s %s: %s\n", finding.Severity, finding.Kind, finding.Path, finding.Message)
	}
	if limit < len(report.Findings) {
		fmt.Printf("... %d additional findings omitted; use -format json or -max-findings 0\n", len(report.Findings)-limit)
	}
}
