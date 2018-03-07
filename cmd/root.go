package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/bpineau/cloud-floating-ip/config"
	"github.com/bpineau/cloud-floating-ip/pkg/operation"
	"github.com/bpineau/cloud-floating-ip/pkg/run"
)

var (
	cfgFile  string
	ip       string
	hoster   string
	instance string
	dryrun   bool
	quiet    bool
	project  string
	zone     string
	accessk  string
	secretk  string
)

func newCfiConfig() *config.CfiConfig {
	conf := &config.CfiConfig{
		IP:            viper.GetString("ip"),
		Hoster:        viper.GetString("hoster"),
		Instance:      viper.GetString("instance"),
		DryRun:        viper.GetBool("dry-run"),
		Quiet:         viper.GetBool("quiet"),
		Project:       viper.GetString("project"),
		Zone:          viper.GetString("zone"),
		AwsAccesKeyID: viper.GetString("aws-access-key-id"),
		AwsSecretKey:  viper.GetString("aws-secret-key"),
	}

	if conf.Hoster != "" && conf.Hoster != "gce" && conf.Hoster != "aws" {
		log.Fatalf("Unsupported hosting provider: '%s\n'", conf.Hoster)
	}

	if conf.IP == "" {
		log.Fatalf("No IP provided")
	}

	return conf
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "cloud-floating-ip",
	Short: "Implement a floating IP by modifying GCE or AWS routes.",
	Long: `Implement a floating IP by modifying GCE or AWS routes.
Most of the arguments (except the floating IP address) can be guessed from
instance's metadata (when running from an AWS or GCE instance).
	`,
	Run: func(cmd *cobra.Command, args []string) {
		run.Run(newCfiConfig(), operation.CfiStatus)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func bindPFlag(key string, cmd string) {
	if err := viper.BindPFlag(key, rootCmd.PersistentFlags().Lookup(cmd)); err != nil {
		log.Fatal("Failed to bind cli argument:", err)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is /etc/cloud-floating-ip.yaml)")
	bindPFlag("cfgFile", "cfgFile")

	rootCmd.PersistentFlags().StringVarP(&ip, "ip", "i", "", "IP address")
	bindPFlag("ip", "ip")

	rootCmd.PersistentFlags().StringVarP(&hoster, "hoster", "o", "", "hosting provider (aws or gce)")
	bindPFlag("hoster", "hoster")

	rootCmd.PersistentFlags().StringVarP(&instance, "instance", "t", "", "instance name")
	bindPFlag("instance", "instance")

	rootCmd.PersistentFlags().BoolVarP(&dryrun, "dry-run", "d", false, "dry-run mode")
	bindPFlag("dry-run", "dry-run")

	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "quiet mode")
	bindPFlag("quiet", "quiet")

	rootCmd.PersistentFlags().StringVarP(&project, "project", "p", "", "GCP project id")
	bindPFlag("project", "project")

	rootCmd.PersistentFlags().StringVarP(&zone, "zone", "z", "", "zone name")
	bindPFlag("zone", "zone")

	rootCmd.PersistentFlags().StringVarP(&accessk, "aws-access-key-id", "a", "", "AWS access key Id")
	bindPFlag("accessk", "accessk")

	rootCmd.PersistentFlags().StringVarP(&secretk, "aws-secret-key", "k", "", "AWS secret key")
	bindPFlag("secretk", "secretk")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath("/etc")
		viper.SetConfigName("cloud-floating-ip")
	}

	// allow config params through prefixed env variables
	viper.SetEnvPrefix("CFI")
	replacer := strings.NewReplacer("-", "_", ".", "_DOT_")
	viper.SetEnvKeyReplacer(replacer)
	viper.AutomaticEnv()

	// If a config file is found, read it in.
	err := viper.ReadInConfig()
	_ = err
}
