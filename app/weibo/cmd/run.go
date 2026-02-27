package cmd

import (
	"github.com/Drelf2018/exp/app/weibo/run"
	"github.com/spf13/cobra"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:     "run",
	Short:   "轮询获取微博",
	Long:    "执行命令后，程序会打开一个无头浏览器窗口用于刷新登录态，并且轮询获取微博",
	Version: "0.0.1",
	RunE:    func(cmd *cobra.Command, args []string) error { return run.Run(cmd.Context(), options) },
}

func init() {
	rootCmd.AddCommand(runCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// runCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// runCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
