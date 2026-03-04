package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Drelf2018/exp/app/weibo/config"
	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "weibo",
	Short: "微博监控",
	Long:  "在完成登录后，程序会定时刷新登录态并轮询获取微博",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// 判断文件格式是否支持
		ext := filepath.Ext(cfgFile)
		var tagName string
		switch strings.ToLower(ext) {
		case ".yaml", ".yml":
			tagName = "yaml"
		case ".json":
			tagName = "json"
		case ".toml":
			tagName = "toml"
		default:
			return fmt.Errorf("不支持的配置文件格式: %q", ext)
		}
		// 检查配置文件是否存在，不存在则输出一份默认配置文件
		_, err := os.Stat(cfgFile)
		if err != nil {
			if os.IsNotExist(err) {
				if err := config.Write(cfgFile, config.Default()); err != nil {
					return fmt.Errorf("无法创建默认配置文件: %w", err)
				}
				return fmt.Errorf("配置文件不存在，已创建默认配置文件: %q ，请修改后重新运行", cfgFile)
			}
			return err
		}
		// 读取配置文件
		v := viper.New()
		v.SetConfigFile(cfgFile)
		if err := v.ReadInConfig(); err != nil {
			return err
		}
		return v.Unmarshal(&options, viper.DecoderConfigOption(func(dc *mapstructure.DecoderConfig) {
			dc.TagName = tagName
		}))
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
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "config.toml", "config file")
}
