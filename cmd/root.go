/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"
	"time"

	"github.com/moolen/flux-poc/pkg/installer"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "flux-poc",
	Short: "",
	Run: func(cmd *cobra.Command, args []string) {

		logrus.SetLevel(logrus.DebugLevel)
		f := installer.New()

		if err := f.Prepare(); err != nil {
			logrus.Fatalf("Error preparing installer: %v", err)
		}

		logrus.Debugf("Checking prerequisites...")
		if err := f.CheckPrerequisites(); err != nil {
			logrus.Warnf("Prerequisite checks failed: %v", err)
		}

		for {
			logrus.Debugf("Reconciling cluster infrastructure...")
			retry, err := f.ReconcileInfrastructure()
			if err != nil && !retry {
				logrus.Fatalf("Error reconciling cluster: %v", err)
			}
			if !retry {
				break
			}
			<-time.After(time.Second * 5)
		}

		f.WithCACert("foobar")
		if err := f.ApplyManifests(); err != nil {
			logrus.Fatalf("Error applying manifests: %v", err)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
