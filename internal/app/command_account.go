package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
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
	amount := fs.Float64("amount", 0, "充值金额，单位元；传入后直接创建微信支付订单")
	planID := fs.Int("plan-id", 0, "套餐 ID；传入后直接创建套餐订阅订单")
	paymentMethod := fs.String("payment-method", defaultPaymentMethod, "支付方式，默认 custom1_wxpay")
	qrImage := fs.Bool("qr-image", true, "将支付二维码保存为图片并打开")
	qrFile := fs.String("qr-file", "", "将支付二维码保存到指定 PNG 文件")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if *amount > 0 && *planID > 0 {
		return errors.New("-amount and -plan-id cannot be used together")
	}
	if err := persistConfigFlags(fs, cfg); err != nil {
		return err
	}
	if *amount <= 0 && *planID <= 0 && !*printOnly {
		c, err := opts.newClient()
		if err != nil {
			return err
		}
		defer c.Close()
		info, err := c.getRechargeInfo(context.Background())
		if err != nil {
			return err
		}
		printRechargeInfo(info)
		return nil
	}
	if *amount > 0 || *planID > 0 {
		c, err := opts.newClient()
		if err != nil {
			return err
		}
		defer c.Close()
		var payment *paymentData
		if *amount > 0 {
			payment, err = c.createRechargePayment(context.Background(), *amount, strings.TrimSpace(*paymentMethod))
		} else {
			payment, err = c.createSubscriptionPayment(context.Background(), *planID, strings.TrimSpace(*paymentMethod))
		}
		if err != nil {
			return err
		}
		if err := printPayment(payment, *qrImage, strings.TrimSpace(*qrFile)); err != nil {
			return err
		}
		return nil
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

func printRechargeInfo(info *rechargeInfo) {
	fmt.Println("充值金额:")
	for _, amount := range info.AmountOptions {
		discount := info.Discount[formatAmountKey(amount)]
		if discount > 0 && discount < 1 {
			fmt.Printf("  %s 元，折扣 %.0f%%\n", formatMoney(amount), discount*100)
		} else {
			fmt.Printf("  %s 元\n", formatMoney(amount))
		}
	}
	if len(info.Plans) > 0 {
		plans := append([]rechargePlan(nil), info.Plans...)
		sort.SliceStable(plans, func(i, j int) bool {
			if plans[i].SortOrder != plans[j].SortOrder {
				return plans[i].SortOrder < plans[j].SortOrder
			}
			return plans[i].ID < plans[j].ID
		})
		fmt.Println("订阅套餐:")
		for _, plan := range plans {
			if !plan.Enabled {
				continue
			}
			subtitle := strings.TrimSpace(plan.Subtitle)
			if subtitle != "" {
				subtitle = " - " + subtitle
			}
			fmt.Printf("  编号 %d: %s%s,%s%s/%s,额度 %s\n",
				plan.ID,
				plan.Title,
				subtitle,
				//plan.Currency,
				"￥",
				formatMoney(plan.PriceAmount),
				formatDuration(plan.DurationValue, plan.DurationUnit),
				formatQuotaAmount(plan.TotalAmount),
			)
		}
	}
	fmt.Println()
	fmt.Println("直接充值: kuaima_cli recharge -amount 100")
	fmt.Println("订阅套餐: kuaima_cli recharge -plan-id 2")
}

func printPayment(payment *paymentData, qrImage bool, qrFile string) error {
	if strings.TrimSpace(payment.TradeNo) != "" {
		fmt.Printf("订单号: %s\n", payment.TradeNo)
	}
	fmt.Printf("支付链接: %s\n", payment.ScanCodeURL)
	if qrImage || qrFile != "" {
		path, err := savePaymentQRCode(payment, qrFile)
		if err != nil {
			return err
		}
		fmt.Printf("支付二维码图片: %s\n", path)
		if qrImage {
			if err := openFile(path); err != nil {
				return fmt.Errorf("open QR image: %w", err)
			}
		}
		return nil
	}
	fmt.Println("请使用微信扫码支付:")
	if err := printQRCode(os.Stdout, payment.ScanCodeURL); err != nil {
		fmt.Printf("二维码生成失败: %v\n", err)
	}
	return nil
}

func savePaymentQRCode(payment *paymentData, qrFile string) (string, error) {
	path := qrFile
	if path == "" {
		tradeNo := strings.TrimSpace(payment.TradeNo)
		if tradeNo == "" {
			tradeNo = "payment"
		}
		path = fmt.Sprintf("kuaima_pay_%s.png", safeFilePart(tradeNo))
	}
	if err := writeQRCodePNG(path, payment.ScanCodeURL, 256); err != nil {
		return "", err
	}
	return path, nil
}

func safeFilePart(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "payment"
	}
	return out
}

func formatAmountKey(amount float64) string {
	if amount == float64(int64(amount)) {
		return fmt.Sprintf("%d", int64(amount))
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", amount), "0"), ".")
}

func formatMoney(amount float64) string {
	return formatAmountKey(amount)
}

func formatDuration(value int, unit string) string {
	if value <= 0 {
		return unit
	}
	switch unit {
	case "month":
		unit = "月"
	case "day":
		unit = "天"
	case "year":
		unit = "年"
	case "week":
		unit = "周"
	case "hour":
		unit = "小时"
	}
	return fmt.Sprintf("%d%s", value, unit)
}
