package auth

type ApiKeyRequest struct {
	ApiKey string `json:"apiKey"`
}

type ApiKeyResponse struct {
	Token string `json:"token"`
}
