package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
)

// test_unsplash 對應 main.go 中的 case "unsplash"
func test_unsplash() {
	apiKey := os.Getenv("UNSPLASH_ACCESS_KEY")
	if apiKey == "" {
		log.Fatal("❌ 錯誤: 環境變數中找不到 UNSPLASH_ACCESS_KEY")
	}

	query := "Kyoto" // 測試搜尋京都
	fmt.Printf("🔍 正在測試 Unsplash API，搜尋關鍵字: %s ...\n", query)

	// 組裝 URL
	endpoint := fmt.Sprintf("https://api.unsplash.com/search/photos?query=%s&per_page=1", url.QueryEscape(query))

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Authorization", "Client-ID "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("❌ 連線失敗: %v", err)
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		log.Fatalf("❌ API 回傳錯誤 (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Results []struct {
			Urls map[string]string `json:"urls"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		log.Fatal("❌ JSON 解析失敗:", err)
	}

	if len(result.Results) > 0 {
		imgURL := result.Results[0].Urls["regular"]
		fmt.Println("✅ 測試成功！圖片網址：")
		fmt.Println(imgURL)
	} else {
		fmt.Println("⚠️ 請求成功，但沒有找到圖片。")
	}
}
