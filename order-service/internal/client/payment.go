package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
)

type HTTPPaymentClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewHTTPPaymentClient(baseURL string, httpClient *http.Client) *HTTPPaymentClient {
	return &HTTPPaymentClient{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

type paymentRequest struct {
	OrderID string `json:"order_id"`
	Amount  int64  `json:"amount"`
}

type paymentResponse struct {
	Status        string `json:"status"`
	TransactionID string `json:"transaction_id"`
}

func (c *HTTPPaymentClient) AuthorizePayment(orderID string, amount int64) (string, string, error) {
	body := paymentRequest{
		OrderID: orderID,
		Amount:  amount,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return "", "", err
	}

	resp, err := c.httpClient.Post(c.baseURL+"/payments", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return "", "", errors.New("payment service is not available")
	}
	defer resp.Body.Close()

	var result paymentResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", err
	}

	return result.Status, result.TransactionID, nil
}
