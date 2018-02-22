package quota

type QuotaRequest struct {
	Identifier string `json:"identifier"`
	Weight     int64  `json:"weight"`
	Interval   int64  `json:"interval"`
	Allow      int64  `json:"allow"`
	TimeUnit   string `json:"timeUnit"`
}

type QuotaResult struct {
	Allowed    int64 `json:"allowed"`
	Used       int64 `json:"used"`
	Exceeded   int64 `json:"exceeded"`
	ExpiryTime int64 `json:"expiryTime"`
	Timestamp  int64 `json:"timestamp"`
}
