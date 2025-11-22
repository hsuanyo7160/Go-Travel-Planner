package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
)

type NominatimResult struct {
	DisplayName string `json:"display_name"`
	Lat         string `json:"lat"`
	Lon         string `json:"lon"`
}

func test_nominatim() {
	city := "Eiffel tower" // 想測別的城市就改這裡

	endpoint := "https://nominatim.openstreetmap.org/search"

	params := url.Values{}
	params.Set("q", city)
	params.Set("format", "json")
	params.Set("limit", "1")

	reqURL := endpoint + "?" + params.Encode()

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		log.Fatal("建立請求失敗:", err)
	}

	// Nominatim 官方要求一定要有 User-Agent
	req.Header.Set("User-Agent", "Go-Travel-Tester/1.0 (gotest924@gmail.com)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal("呼叫 Nominatim 失敗:", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Nominatim 回傳非 200，status=%s\n", resp.Status)
	}

	var results []NominatimResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		log.Fatal("解析 Nominatim 回應失敗:", err)
	}

	if len(results) == 0 {
		fmt.Println("找不到城市，results 為空")
		return
	}

	fmt.Println("=== Nominatim 測試成功 ===")
	fmt.Println("查詢城市：", city)
	fmt.Println("完整地名：", results[0].DisplayName)
	fmt.Println("緯度：", results[0].Lat)
	fmt.Println("經度：", results[0].Lon)
}
