package analytics

type PeriodResponse struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type ComparisonResponse struct {
	IncomeChangePct  float64 `json:"income_change_pct"`
	ExpenseChangePct float64 `json:"expense_change_pct"`
}

type SummaryResponse struct {
	Period           PeriodResponse     `json:"period"`
	TotalIncome      int                `json:"total_income"`
	TotalExpense     int                `json:"total_expense"`
	Net              int                `json:"net"`
	TotalBalance     int                `json:"total_balance"`
	TransactionCount int                `json:"transaction_count"`
	Comparison       ComparisonResponse `json:"comparison"`
}

type CategoryBreakdownResponse struct {
	CategoryID   string  `json:"category_id"`
	CategoryName string  `json:"category_name"`
	CategoryType string  `json:"category_type"`
	IsDeleted    bool    `json:"is_deleted"`
	Total        int     `json:"total"`
	Count        int     `json:"count"`
	Percentage   float64 `json:"percentage"`
}

type WalletBreakdownResponse struct {
	WalletID   string `json:"wallet_id"`
	WalletName string `json:"wallet_name"`
	IsDeleted  bool   `json:"is_deleted"`
	Income     int    `json:"income"`
	Expense    int    `json:"expense"`
	Net        int    `json:"net"`
	Count      int    `json:"count"`
}

type TrendBucketResponse struct {
	Bucket  string `json:"bucket"`
	Start   string `json:"start"`
	End     string `json:"end"`
	Income  int    `json:"income"`
	Expense int    `json:"expense"`
	Net     int    `json:"net"`
}
