package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func sign(params string, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(params))
	return hex.EncodeToString(mac.Sum(nil))
}

func placeLimitOrder(apiKey, apiSecret, symbol, side, quantity, price string) error {
	endpoint := "https://api.mexc.com/api/v3/order"

	// Подготовка параметров
	recvWindow := "5000"
	timestamp := strconv.FormatInt(time.Now().UnixNano()/1e6, 10)
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("side", side) // "BUY" или "SELL"
	params.Set("type", "LIMIT")
	params.Set("quantity", quantity)
	params.Set("price", price)
	params.Set("recvWindow", recvWindow)
	params.Set("timestamp", timestamp)

	// Подпись
	queryString := params.Encode()
	signature := sign(queryString, apiSecret)
	params.Set("signature", signature)

	// Формируем HTTP-запрос
	req, err := http.NewRequest("POST", endpoint, io.NopCloser(strings.NewReader(params.Encode())))
	if err != nil {
		return err
	}
	req.Header.Set("X-MEXC-APIKEY", apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	fmt.Printf("Response status: %s\n", resp.Status)
	fmt.Printf("Response body: %s\n", string(bodyBytes))

	return nil
}

func main() {
	apiKey := "mx0vglHWdyJdxCoTtc"
	apiSecret := "b36cbfefc7ac4b79a0ece98ff0415e22"
	symbol := "DOLZUSDT"
	side := "BUY"
	quantity := "1799.0"
	price := "0.00556"

	if err := placeLimitOrder(apiKey, apiSecret, symbol, side, quantity, price); err != nil {
		fmt.Printf("Order failed: %v\n", err)
	}
}
