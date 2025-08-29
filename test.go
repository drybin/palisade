package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"
)

func main() {
	apiKey := os.Getenv("MEXC_API_KEY")     // положи ключ в ENV
	secretKey := os.Getenv("MEXC_API_SECRET")

	baseURL := "https://api.mexc.com"
	endpoint := "/api/v3/account"

	// добавляем timestamp
	timestamp := time.Now().UnixMilli()

	// параметры запроса
	params := url.Values{}
	params.Set("timestamp", fmt.Sprintf("%d", timestamp))

	// считаем подпись
	signature := sign(params.Encode(), secretKey)
	params.Set("signature", signature)

	// формируем URL
	fullURL := baseURL + endpoint + "?" + params.Encode()

	// создаем запрос
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("X-MEXC-APIKEY", apiKey)

	// выполняем запрос
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println(string(body))
}

// функция генерации подписи HMAC-SHA256
func sign(message, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}
