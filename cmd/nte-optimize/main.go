package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"nte-optimizer/internal/app"
	"nte-optimizer/internal/nte"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("nte-optimize", flag.ContinueOnError)
	character := flags.String("character", "", "numeric character ID or profile ID")
	profile := flags.String("profile", "", "specific profile ID when a character has multiple profiles")
	method := flags.String("method", "fast", "search method: fast or beta")
	language := flags.String("lang", "en", "game-label language: en or fr")
	stateDir := flags.String("state-dir", defaultStateDir(), "user data directory containing workspace/")
	dataDir := flags.String("data", "data", "project data directory")
	experimentsFile := flags.String("experiments", "", "run named one-shot variants from a JSON experiment file")
	jsonOutput := flags.Bool("json", false, "print the result as JSON (full inventory details unless --experiments is set)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if *method != "fast" && *method != "beta" {
		return fmt.Errorf("invalid method %q: choose fast or beta", *method)
	}
	if *language != "en" && *language != "fr" {
		return fmt.Errorf("invalid language %q: choose en or fr", *language)
	}
	if *character == "" && *profile == "" {
		return errors.New("specify --character or --profile")
	}
	service := app.NewOptimizerService(*dataDir)
	request, err := app.PrepareSavedOptimization(service, *stateDir, *character, *profile)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if *experimentsFile != "" {
		plan, err := app.ReadExperimentPlan(*experimentsFile)
		if err != nil {
			return err
		}
		report, err := app.RunSavedOptimizationExperiments(ctx, service, *stateDir, *language, *method, request, plan)
		if err != nil {
			return err
		}
		if *jsonOutput {
			err = json.NewEncoder(output).Encode(report)
		} else {
			err = printExperimentReport(output, report)
		}
		if err != nil {
			return err
		}
		return ctx.Err()
	}
	started := time.Now()
	result, err := app.OptimizeSavedProject(ctx, service, *stateDir, *language, *method, request)
	if err != nil {
		return err
	}
	if *jsonOutput {
		return json.NewEncoder(output).Encode(result)
	}
	return printReport(output, request, result, time.Since(started))
}

func printExperimentReport(output io.Writer, report app.ExperimentReport) error {
	w := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
	fmt.Fprintf(w, "%s:\t%s (%s · #%d)\n", report.Text.Profile, report.Profile.Name, report.Profile.ID, report.Profile.CharacterID)
	sharedVariants := strings.Replace(report.Text.SharedVariants, "{count}", fmt.Sprint(len(report.Variants)), 1)
	fmt.Fprintf(w, "%s:\t%s\n", report.Text.InputSnapshot, sharedVariants)
	fmt.Fprintln(w, report.Text.ActionDamageNote)
	fmt.Fprintln(w, report.Text.RotationNote)
	fmt.Fprintln(w, report.Text.ScoreScaleNote)
	actionNames := map[string]string{}
	actionIDs := []string{}
	seen := map[string]bool{}
	for _, variant := range report.Variants {
		for _, action := range variant.ActionDamage {
			actionNames[action.ID] = action.Name
			if !seen[action.ID] {
				seen[action.ID] = true
				actionIDs = append(actionIDs, action.ID)
			}
		}
	}
	sort.Strings(actionIDs)
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s Δ", report.Text.Variant, report.Text.Method, report.Text.Status, report.Text.RetainedEligible, report.Text.Visited, report.Text.Duration, report.Text.Arc, report.Text.BasicDamage)
	for _, id := range actionIDs {
		fmt.Fprintf(w, "\t%s Δ", actionNames[id])
	}
	fmt.Fprintln(w)
	for _, variant := range report.Variants {
		arc := "-"
		if variant.EffectiveConfig.Arc != nil {
			arc = fmt.Sprintf("%s %s %d", variant.EffectiveConfig.Arc.Name, report.Text.Level, variant.EffectiveConfig.Arc.Level)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d/%d\t%d\t%dms\t%s", variant.Name, variant.EffectiveConfig.Method, variant.Status, variant.SelectedCandidates, variant.EligibleCandidates, variant.VisitedStates, variant.DurationMS, arc)
		if variant.BasicDamage == nil || variant.BasicDamage.GainPercent == nil {
			fmt.Fprint(w, "\t-")
		} else {
			fmt.Fprintf(w, "\t%+.1f%%", *variant.BasicDamage.GainPercent)
		}
		actions := map[string]app.ExperimentActionDamage{}
		for _, action := range variant.ActionDamage {
			actions[action.ID] = action
		}
		for _, id := range actionIDs {
			action, ok := actions[id]
			if !ok || action.GainPercent == nil {
				fmt.Fprint(w, "\t-")
			} else {
				fmt.Fprintf(w, "\t%+.1f%%", *action.GainPercent)
			}
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, report.Text.ScoresNotCompared)
	return w.Flush()
}

func defaultStateDir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base, _ = os.UserConfigDir()
	}
	return filepath.Join(base, "NTE Optimizer")
}

func printReport(output io.Writer, request app.SavedOptimizationRequest, result app.OptimizationResult, elapsed time.Duration) error {
	w := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
	fmt.Fprintf(w, "Profile:\t%s (%s, character %d)\n", request.Profile.Name, request.Profile.ID, request.Profile.CharacterID)
	fmt.Fprintf(w, "Method:\t%s\n", result.OptimizationMode)
	fmt.Fprintf(w, "Search:\t%s; %d visited states; %d selected candidates; %s\n", searchStatus(result.Solution.Complete), result.Solution.Visited, result.SelectedCandidates, elapsed.Round(time.Millisecond))
	fmt.Fprintf(w, "Main stats:\t%s\n", strings.Join(request.Weights.MainStats, ", "))
	fmt.Fprintln(w, "\nOBJECTIVES (saved settings)")
	fmt.Fprintln(w, "Statistic\tTarget\tWeight\tStrict min\tFinal\tPoints")
	keys := make([]string, 0, len(request.Goals))
	for key := range request.Goals {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	points := map[string]float64{}
	if result.Solution.Ranking != nil {
		for _, item := range result.Solution.Ranking.Contributions {
			points[item.PropertyID] += item.Points
		}
	}
	for _, key := range keys {
		goal := request.Goals[key]
		strict := "-"
		if goal.StrictMinimum {
			strict = fmt.Sprintf("%.4f", goal.Minimum)
		}
		fmt.Fprintf(w, "%s\t%.4f\t%.2f\t%s\t%.4f\t%.4f\n", key, goal.Target, goal.Importance, strict, result.Stats.Derived[key], points[key])
	}
	fmt.Fprintln(w, "\nRANKING")
	if ranking := result.Solution.Ranking; ranking != nil {
		fmt.Fprintf(w, "Objective fit:\t%.6f\nEquipment relevance (diagnostic):\t%.6f\nEquipment tie-break:\t%.6f\nTotal:\t%.6f\n", ranking.Objectives, ranking.Equipment, ranking.TieBreak, ranking.Score)
	} else {
		fmt.Fprintf(w, "Total:\t%.6f\n", result.Solution.Score)
	}
	fmt.Fprintf(w, "Basic DMG index:\t%.1f\n", result.Stats.Derived["BasicDamageIndex"])
	fmt.Fprintln(w, "\nEQUIPMENT")
	fmt.Fprintf(w, "Set:\t%s (%s); diagnostic relevance %.2f\n", result.Set.Name, result.Set.ID, result.Solution.SetBonusScore)
	if result.Cartridge != nil {
		fmt.Fprintf(w, "Cartridge:\t%s; level %d; relevance %.2f; main [%s]; secondary [%s]\n", result.Cartridge.LocalID, result.Cartridge.Level, result.Solution.CartridgeScore, formatStats(result.Cartridge.MainStats), formatStats(result.Cartridge.SubStats))
	}
	for index, module := range result.Modules {
		fmt.Fprintf(w, "Module %d:\t%s; %s; level %d; relevance %.2f; main [%s]; secondary [%s]\n", index+1, module.Module.LocalID, module.Module.Geometry, module.Module.Level, module.Breakdown.Total, formatStats(module.Module.MainStats), formatStats(module.Module.SubStats))
	}
	fmt.Fprintln(w, "\nDAMAGE PREVIEW")
	if result.Damage == nil || len(result.Damage.Groups) == 0 {
		fmt.Fprintln(w, "Unavailable")
	} else {
		fmt.Fprintf(w, "Status:\t%s\n", result.Damage.Status)
		fmt.Fprintln(w, "Action\tBuild average\tBuild crit\tCurrent average\tGain")
		for _, group := range result.Damage.Groups {
			gain := "-"
			if group.CurrentDamage > 0 {
				gain = fmt.Sprintf("%+.1f%%", (group.BuildDamage/group.CurrentDamage-1)*100)
			}
			fmt.Fprintf(w, "%s\t%.1f\t%.1f\t%.1f\t%s\n", group.Name, group.BuildDamage, group.BuildCrit, group.CurrentDamage, gain)
		}
	}
	return w.Flush()
}

func formatStats(stats []nte.Stat) string {
	items := make([]string, 0, len(stats))
	for _, stat := range stats {
		if stat.Percent {
			items = append(items, fmt.Sprintf("%s %.1f%%", stat.PropertyID, stat.Value*100))
		} else {
			items = append(items, fmt.Sprintf("%s %.1f", stat.PropertyID, stat.Value))
		}
	}
	return strings.Join(items, ", ")
}

func searchStatus(complete bool) string {
	if complete {
		return "complete"
	}
	return "approximate; optimality not guaranteed"
}
