package app

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

func runLogin(args []string) error {
	cfg, err := loadAppConfig()
	if err != nil {
		return err
	}
	fs := newFlagSet("login")
	opts := addClientFlagsWithConfig(fs, cfg)
	force := fs.Bool("force", false, "重新登录并刷新 API key")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := persistConfigFlags(fs, cfg); err != nil {
		return err
	}

	apiKey := strings.TrimSpace(*opts.apiKey)
	if apiKey == "" || *force {
		logFile, err := openLogFile(*opts.logFile)
		if err != nil {
			return err
		}
		var logWriter io.Writer
		if logFile != nil {
			defer logFile.Close()
			logWriter = logFile
		}
		apiKey, err = ensureAPIKey(context.Background(), *opts.baseURL, *opts.username, *opts.password, *opts.verbose, logWriter)
		if err != nil {
			return err
		}
	}
	fmt.Println(apiKey)
	return nil
}

func runBalance(args []string) error {
	cfg, err := loadAppConfig()
	if err != nil {
		return err
	}
	fs := newFlagSet("balance")
	opts := addClientFlagsWithConfig(fs, cfg)
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := persistConfigFlags(fs, cfg); err != nil {
		return err
	}
	c, err := opts.newClient()
	if err != nil {
		return err
	}
	defer c.Close()
	usage, err := c.getTokenUsage(context.Background())
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", usage.Name)
	fmt.Printf("可用额度: %s\n", formatQuotaAmount(usage.TotalAvailable))
	fmt.Printf("已消耗: %s\n", formatQuotaAmount(usage.TotalUsed))
	if len(usage.Subscriptions) > 0 {
		fmt.Println("订阅套餐:")
		for _, item := range usage.Subscriptions {
			sub := item.Subscription
			fmt.Printf("    状态: %s\n", sub.Status)
			fmt.Printf("    可用额度: %s\n", formatQuotaAmount(sub.AmountTotal-sub.AmountUsed))
			fmt.Printf("    已消耗: %s\n", formatQuotaAmount(sub.AmountUsed))
			fmt.Printf("    订阅时间: %s\n", formatTime(sub.StartTime))
			fmt.Printf("    到期时间: %s\n", formatTime(sub.EndTime))
			//fmt.Printf("    last_reset_time: %d\n", sub.LastResetTime)
			//fmt.Printf("    next_reset_time: %d\n", sub.NextResetTime)
		}
	}
	return nil
}

func formatQuotaAmount(quota int64) string {
	return fmt.Sprintf("%d (￥%.2f)", quota, float64(quota)/500000)
}
func formatTime(ts int64) string {
	if ts <= 0 {
		return "-"
	}
	tm := time.Unix(ts, 0)
	return tm.Format("2006-01-02 15:04")
}

func runRecharge(args []string) error {
	cfg, err := loadAppConfig()
	if err != nil {
		return err
	}
	fs := newFlagSet("recharge")
	opts := addClientFlagsWithConfig(fs, cfg)
	printOnly := fs.Bool("print-url", false, "只打印充值页面 URL，不打开浏览器")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := persistConfigFlags(fs, cfg); err != nil {
		return err
	}
	logFile, err := openLogFile(*opts.logFile)
	if err != nil {
		return err
	}
	var logWriter io.Writer
	if logFile != nil {
		defer logFile.Close()
		logWriter = logFile
	}
	username, password, err := resolveRechargeCredentials(context.Background(), *opts.baseURL, *opts.username, *opts.password, *opts.verbose, logWriter)
	if err != nil {
		return err
	}
	rawURL, err := rechargeURL(*opts.baseURL, username, password, time.Now())
	if err != nil {
		return err
	}
	if *printOnly {
		fmt.Println(rawURL)
		return nil
	}
	if err := openBrowser(rawURL); err != nil {
		return err
	}
	fmt.Println(rawURL)
	return nil
}
