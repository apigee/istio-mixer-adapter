package quota

import (
	"bytes"
	"encoding/json"
	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"istio.io/istio/mixer/pkg/adapter"
	"net/http"
	"path"
	"strconv"
	"fmt"
)

// todo: support args.DeduplicationID, args.BestEffort

const quotaPath = "/quotas/organization/%s/environment/%s"

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

func Apply(auth auth.Context, product auth.ApiProductDetails, args adapter.QuotaArgs) (QuotaResult, error) {

	quotaURL := auth.ApigeeBase()
	quotaURL.Path = path.Join(quotaURL.Path, fmt.Sprintf(quotaPath, auth.Organization(), auth.Environment()))

	allow, err := strconv.ParseInt(product.QuotaLimit, 10, 64)
	if err != nil {
		return QuotaResult{}, err
	}

	// todo: hold up, if it's per app, it's not per product, right?
	// todo: I feel like the Identifier should be app+product?
	request := QuotaRequest{
		Identifier: auth.Application,
		Weight:     args.QuotaAmount,
		Interval:   product.QuotaInterval,
		Allow:      allow,
		TimeUnit:   product.QuotaTimeUnit,
	}

	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(request)

	req, err := http.NewRequest(http.MethodPost, quotaURL.String(), body)
	if err != nil {
		return QuotaResult{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	auth.Log().Infof("Sending to (%s): %s\n", quotaURL.String(), body)

	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return QuotaResult{}, err
	}
	defer resp.Body.Close()

	buf := bytes.NewBuffer(make([]byte, 0, resp.ContentLength))
	_, err = buf.ReadFrom(resp.Body)
	respBody := buf.Bytes()

	switch resp.StatusCode {
	case 200:
		var quotaResult QuotaResult
		if err = json.Unmarshal(respBody, &quotaResult); err != nil {
			err = auth.Log().Errorf("Error unmarshalling: %s\n", string(respBody))
		}
		return quotaResult, err

	default:
		return QuotaResult{}, auth.Log().Errorf("quota apply failed. result: %s\n", string(respBody))
	}
}

type ErrorResponse struct {
	ErrorCode string `json:"errorCode"`
	Reason    string `json:"reason"`
}
