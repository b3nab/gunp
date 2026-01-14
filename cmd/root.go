package cmd

import (
	"gunp/internal/app"
	logger "gunp/internal/log"
	"os"

	"github.com/spf13/cobra"
)

func init() {
	// rootCmd.PersistentFlags().StringP("path", "p", "", "use a different directory instead of the cwd")
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gunp",
	Short: "Recursively scan git repos for unpushed commits with a nice Terminal UI",
	Long: `gunp stands for Git UNPublished.

Recursively scan git repos for unpushed commits with a nice Terminal UI
`,
	// Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// path := args[0]
		app.StartUnpushedApp()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logger.Get().Error("rootCmd.Execute", "err", err)
		os.Exit(1)
	}
}
