package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/playwright-community/playwright-go"
	"github.com/spf13/cobra"
)

// loginCmd represents the login command
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "扫码登录",
	Long:  "执行命令后，程序会打开一个有头浏览器窗口，你需要在这个浏览器完成登录。准备就绪后创建一个新空白窗口，程序会自动保存此时的用户信息数据到本地文件",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 初始化浏览器
		err := playwright.Install()
		if err != nil {
			return fmt.Errorf("安装 playwright 失败: %w", err)
		}
		pw, err := playwright.Run()
		if err != nil {
			return fmt.Errorf("启动 playwright 失败: %w", err)
		}
		opts := playwright.BrowserTypeLaunchOptions{
			Args: []string{
				"--no-sandbox",
				"--disable-dev-shm-usage",
				"--disable-features=AutomationControlled",
				"--disable-blink-features=AutomationControlled", // 移除自动化控制特征
				"--disable-features=IsolateOrigins,site-per-process",
				"--disable-features=BlockInsecurePrivateNetworkRequests",
				"--window-size=1920,1080", // 模拟有头窗口大小
				"--start-maximized",
			},
			Headless: playwright.Bool(false),
		}
		browser, err := pw.Chromium.Launch(opts)
		if err != nil {
			return fmt.Errorf("启动 Chromium 浏览器失败: %w", err)
		}
		defer browser.Close()
		// 新建上下文
		browserContext, err := browser.NewContext()
		if err != nil {
			return fmt.Errorf("获取浏览器上下文失败: %w", err)
		}
		defer browserContext.Close()
		// 新建页面
		page, err := browserContext.NewPage()
		if err != nil {
			return fmt.Errorf("新建页面失败: %w", err)
		}
		defer page.Close()
		// 访问微博主页登录
		_, err = page.Goto("https://weibo.com")
		if err != nil {
			return fmt.Errorf("访问微博主页失败: %w", err)
		}
		// 等待登录
		disconnected := make(chan struct{})
		browserContext.OnPage(func(p playwright.Page) {
			if p.URL() == "chrome://newtab/" {
				close(disconnected)
			}
		})
		<-disconnected
		// 保存状态
		storage, err := browserContext.StorageState()
		if err != nil {
			return fmt.Errorf("获取浏览器状态失败: %w", err)
		}
		b, err := json.Marshal(storage)
		if err != nil {
			return fmt.Errorf("序列化浏览器状态失败: %w", err)
		}
		err = os.WriteFile(options.State, b, os.ModePerm)
		if err != nil {
			return fmt.Errorf("写入浏览器状态失败: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// loginCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// loginCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
