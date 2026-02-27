package cmd

import (
	"os"

	"github.com/Drelf2018/exp/app/weibo/config"
	"github.com/jessevdk/go-flags"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "weibo",
	Short: "微博监控",
	Long:  "在完成登录后，程序会定时刷新登录态并轮询获取微博",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		p := flags.NewParser(&options, flags.Default)
		_, err := p.ParseArgs(args)
		if err != nil {
			return err
		}
		return flags.NewIniParser(p).ParseFile(cfgFile)
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

var cfgFile string
var options config.Options

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "config.ini", "config file (default is $HOME/.weibo.yaml)")
}
