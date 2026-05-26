package app

import (
	"encoding/json"
	"fmt"
	"strings"
)

const rechargeHint = "余额可能不足，请运行 kuaima_cli recharge 充值。"

func apiErrorf(format string, args ...any) error {
	message := fmt.Sprintf(format, args...)
	return fmt.Errorf("%s", appendRechargeHint(message))
}

func apiErrorMessage(err *apiError) string {
	if err == nil {
		return appendRechargeHint("API error")
	}
	message := strings.TrimSpace(err.Message)
	if message == "" {
		message = "API error"
	}
	haystack := message + " " + strings.TrimSpace(err.Type) + " " + apiErrorCodeString(err.Code)
	return appendRechargeHintFor(haystack, message)
}

func apiRequestError(operation, status string, data []byte) error {
	detail := strings.TrimSpace(string(data))
	base := fmt.Sprintf("%s failed: %s", operation, status)
	if detail != "" {
		base += ": " + detail
	}
	return fmt.Errorf("%s", appendRechargeHintFor(detail, base))
}

func appendRechargeHint(message string) string {
	return appendRechargeHintFor(message, message)
}

func appendRechargeHintFor(haystack, message string) string {
	if !looksLikeInsufficientBalance(haystack) || strings.Contains(message, rechargeHint) {
		return message
	}
	return message + "\n" + rechargeHint
}

func looksLikeInsufficientBalance(message string) bool {
	lower := strings.ToLower(message)
	needles := []string{
		"insufficient_quota",
		"insufficient quota",
		"insufficient credits",
		"insufficient balance",
		"out of credits",
		"quota exceeded",
		"balance_not_enough",
		"not enough balance",
		"余额不足",
		"额度不足",
		"额度失败",
		"余额不够",
		"额度不够",
		"欠费",
	}
	for _, needle := range needles {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func apiErrorCodeString(code any) string {
	switch value := code.(type) {
	case nil:
		return ""
	case string:
		return value
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return fmt.Sprint(value)
		}
		return string(data)
	}
}
