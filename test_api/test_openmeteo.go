package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
)

type OpenMeteoResponse struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timezone  string  `json:"timezone"`
	Hourly    struct {
		Time          []string  `json:"time"`
		Temperature2m []float64 `json:"temperature_2m"`
	} `json:"hourly"`
}

func test_openmeteo() {
	// 一樣用台北附近（可以換掉）
	lat := "25.033964"
	lon := "121.564468"

	endpoint := "https://api.open-meteo.com/v1/forecast"

	params := url.Values{}
	params.Set("latitude", lat)
	params.Set("longitude", lon)
	params.Set("hourly", "temperature_2m")
	params.Set("timezone", "auto")
	params.Set("forecast_days", "1")

	reqURL := endpoint + "?" + params.Encode()

	resp, err := http.Get(reqURL)
	if err != nil {
		log.Fatal("呼叫 Open-Meteo 失敗:", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Open-Meteo 回傳非 200，status=%s\n", resp.Status)
	}

	var omResp OpenMeteoResponse
	if err := json.NewDecoder(resp.Body).Decode(&omResp); err != nil {
		log.Fatal("解析 Open-Meteo 回應失敗:", err)
	}

	fmt.Println("=== Open-Meteo 測試結果 ===")
	fmt.Printf("位置：lat=%.4f, lon=%.4f, timezone=%s\n", omResp.Latitude, omResp.Longitude, omResp.Timezone)

	// 只秀前 5 個小時的溫度
	for i := 0; i < 5 && i < len(omResp.Hourly.Time); i++ {
		fmt.Printf("%s -> %.1f°C\n", omResp.Hourly.Time[i], omResp.Hourly.Temperature2m[i])
	}
}
