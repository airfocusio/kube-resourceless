package cmd

import (
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/airfocusio/kube-resourceless/internal"
	"github.com/spf13/cobra"
)

var (
	verbose        bool
	rootCmdTLSCert string
	rootCmdTLSKey  string
	rootCmd        = &cobra.Command{
		Use: "kube-resourceless",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, err := internal.NewService(internal.ServiceOpts{
				TLSCertFile: rootCmdTLSCert,
				TLSKeyFile:  rootCmdTLSKey,
			})
			if err != nil {
				return err
			}

			term := make(chan os.Signal, 1)
			signal.Notify(term, syscall.SIGTERM)
			signal.Notify(term, syscall.SIGINT)
			if err := service.Run(term); err != nil {
				return err
			}
			return nil
		},
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if !verbose {
				internal.Debug = log.New(ioutil.Discard, "", log.LstdFlags)
			}
		},
	}
)

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "")
	rootCmd.Flags().StringVar(&rootCmdTLSCert, "tls-cert", "/etc/certs/tls.crt", "Path to the TLS certificate")
	rootCmd.Flags().StringVar(&rootCmdTLSKey, "tls-key", "/etc/certs/tls.key", "Path to the TLS key")
	rootCmd.AddCommand(versionCmd)
}
