// This file implements the 'ruust create' command (alias 'deploy'): it hatches a
// new Egg from a git repo, branch, region, and tier by calling CreateEgg, then
// prints the new Egg with its url and nudges the user towards 'ruust logs'.
package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/RuustRun/cli/internal/api"
	"github.com/RuustRun/cli/internal/config"
	"github.com/RuustRun/cli/internal/ui"
)

// create flag values.
var (
	createRepo   string
	createBranch string
	createRegion string
	createTier   string
)

// createCmd hatches a new Egg from a git repo.
var createCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"deploy"},
	Short:   "Hatch a new Egg from a git repo",
	Long: "Create an Egg from a git repo and watch it go from incubating to hatched.\n" +
		"Pricing is flat and per Egg with unmetered egress: nano £3, small £6,\n" +
		"standard £12, large £24 per month. Custom sizes on the sliding scale are\n" +
		"available in the dashboard.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if config.Token(cfg) == "" {
			return errNotSignedIn
		}

		if strings.TrimSpace(createRepo) == "" {
			return fmt.Errorf("a git repo is required (pass --repo <git url>)")
		}

		egg, err := Client().CreateEgg(createRepo, createBranch, createRegion, createTier)
		if err != nil {
			return err
		}

		printCreatedEgg(cmd.OutOrStdout(), egg)
		return nil
	},
}

// printCreatedEgg prints a short confirmation for a freshly created Egg: its
// name and state, key details, the url, and a nudge to follow the logs.
func printCreatedEgg(w interface{ Write([]byte) (int, error) }, egg api.Egg) {
	fmt.Fprintln(w, ui.Logo())
	fmt.Fprintln(w)
	fmt.Fprintln(w, ui.Bone.Render("Hatching a new Egg ")+ui.Ember.Render(egg.Name)+
		ui.Bone.Render(", now ")+ui.StateLabel(egg.State)+ui.Bone.Render("."))
	fmt.Fprintln(w)

	fmt.Fprintln(w, ui.Key.Render("  name    ")+ui.Bone.Render(egg.Name))
	fmt.Fprintln(w, ui.Key.Render("  region  ")+ui.Bone.Render(regionCell(egg)))
	fmt.Fprintln(w, ui.Key.Render("  tier    ")+ui.Bone.Render(tierCell(egg))+
		"  "+ui.Subtle.Render(formatGBP(egg.PriceGbp)+" per month, unmetered egress"))
	fmt.Fprintln(w, ui.Key.Render("  state   ")+ui.StateLabel(egg.State))
	if strings.TrimSpace(egg.URL) != "" {
		fmt.Fprintln(w, ui.Key.Render("  url     ")+ui.Ember.Render("https://"+egg.URL))
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, ui.Subtle.Render("Follow it hatch with ")+
		ui.Key.Render("ruust logs "+egg.Name)+ui.Subtle.Render("."))
}

func init() {
	createCmd.Flags().StringVar(&createRepo, "repo", "", "git repo url to deploy (required)")
	createCmd.Flags().StringVar(&createBranch, "branch", "main", "branch to deploy")
	createCmd.Flags().StringVar(&createRegion, "region", "eu-west",
		"region slug: eu-west (London) or us-east (Virginia)")
	createCmd.Flags().StringVar(&createTier, "tier", "standard",
		"tier: nano, small, standard, or large")
	AddCommand(createCmd)
}
