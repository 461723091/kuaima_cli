package app

import (
	"context"
	"fmt"
	"strings"
)

func accountGroupFromClient(ctx context.Context, c *client) string {
	if c == nil {
		return ""
	}
	usage, err := c.getTokenUsage(ctx)
	if err != nil || usage == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(usage.Group))
}

func imageGenerationCountLimit(group string) int {
	switch strings.ToLower(strings.TrimSpace(group)) {
	case "svip":
		return 10
	case "vip":
		return 3
	default:
		return 1
	}
}

func imageGenerationCountLimitMessage(group string, requested int) string {
	limit := imageGenerationCountLimit(group)
	switch strings.ToLower(strings.TrimSpace(group)) {
	case "svip":
		return fmt.Sprintf("SVIP 一次最多生成 %d 张", limit)
	case "vip":
		if requested > limit {
			return fmt.Sprintf("VIP 一次最多生成 %d 张，升级 SVIP 后可一次生成 10 张", limit)
		}
		return fmt.Sprintf("VIP 一次最多生成 %d 张", limit)
	default:
		return "普通用户一次只能生成 1 张，升级 VIP 后可一次生成 3 张"
	}
}
