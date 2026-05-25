package app

type userLoginResponse struct {
	Message string `json:"message"`
	Success bool   `json:"success"`
	Data    struct {
		Token string `json:"token"`
	} `json:"data"`
}

type tokenUsage struct {
	Object         string              `json:"object"`
	Name           string              `json:"name"`
	Subscriptions  []tokenSubscription `json:"subscriptions"`
	TotalGranted   int64               `json:"total_granted"`
	TotalUsed      int64               `json:"total_used"`
	TotalAvailable int64               `json:"total_available"`
	UnlimitedQuota bool                `json:"unlimited_quota"`
	ExpiresAt      int64               `json:"expires_at"`
}

type tokenSubscription struct {
	Subscription subscriptionPlan `json:"subscription"`
}

type subscriptionPlan struct {
	ID            int64  `json:"id"`
	UserID        int64  `json:"user_id"`
	PlanID        int64  `json:"plan_id"`
	AmountTotal   int64  `json:"amount_total"`
	AmountUsed    int64  `json:"amount_used"`
	StartTime     int64  `json:"start_time"`
	EndTime       int64  `json:"end_time"`
	Status        string `json:"status"`
	Source        string `json:"source"`
	LastResetTime int64  `json:"last_reset_time"`
	NextResetTime int64  `json:"next_reset_time"`
	UpgradeGroup  string `json:"upgrade_group"`
	PrevUserGroup string `json:"prev_user_group"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

type tokenUsageResponse struct {
	Code    bool       `json:"code"`
	Message string     `json:"message"`
	Data    tokenUsage `json:"data"`
}
