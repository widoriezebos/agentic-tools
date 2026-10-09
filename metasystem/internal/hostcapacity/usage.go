package hostcapacity

type Tokens struct {
	Calls         int   `json:"calls"`
	Input         int64 `json:"inputTokens"`
	CacheRead     int64 `json:"cacheReadTokens"`
	CacheCreation int64 `json:"cacheCreationTokens"`
	Output        int64 `json:"outputTokens"`
	PeakContext   int64 `json:"peakContext"`
}

type SessionUsage struct {
	Provider         string   `json:"provider"`
	Session          string   `json:"session"`
	WorkingDirectory string   `json:"workingDirectory"`
	Provisional      bool     `json:"provisional"`
	Totals           *Tokens  `json:"totals"`
	TrailingHour     *Tokens  `json:"trailingHour"`
	Problems         []string `json:"problems"`
}

type Usage struct {
	Sessions         []SessionUsage `json:"sessions"`
	From             string         `json:"trailingHourFrom"`
	Until            string         `json:"trailingHourUntil"`
	AccountWindow    *string        `json:"accountWindow"`
	AccountLimit     *int64         `json:"accountLimit"`
	AccountNumerator *int64         `json:"accountNumerator"`
	AccountPercent   *float64       `json:"accountPercent"`
	Problems         []string       `json:"problems"`
}
