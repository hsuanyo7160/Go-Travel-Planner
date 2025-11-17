package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// Overpass 回傳的元素格式（我們只抓幾個常用欄位）
type OverpassElement struct {
	ID   int64             `json:"id"`
	Lat  float64           `json:"lat"`
	Lon  float64           `json:"lon"`
	Tags map[string]string `json:"tags"`
}

type OverpassResponse struct {
	Elements []OverpassElement `json:"elements"`
}

func test_opentripmap() {
	// 一樣用台北 101 附近
	lat := 25.033964
	lon := 121.564468
	radius := 1000 // 公尺

	// Overpass QL：找半徑 X 公尺內有 name 的景點 / 景點類 POI
	// 這裡示範找 tourism=attraction 或 amenity=cafe 的點
	query := fmt.Sprintf(`
[out:json];
(
  node["tourism"="attraction"]["name"](around:%d,%f,%f);
  node["amenity"="cafe"]["name"](around:%d,%f,%f);
);
out 10;
`, radius, lat, lon, radius, lat, lon)

	endpoint := "https://overpass-api.de/api/interpreter"

	log.Println("送到 Overpass 的查詢：")
	log.Println(query)

	// 用 POST，body 傳 data=查詢字串
	req, err := http.NewRequest("POST", endpoint, bytes.NewBufferString("data="+query))
	if err != nil {
		log.Println("建立 Overpass 請求失敗:", err)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Go-Travel-Tester/1.0 (gotest924@gmail.com)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("呼叫 Overpass 失敗:", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		log.Printf("Overpass 回傳非 200，status=%s\n", resp.Status)
		fmt.Println("Raw body:", string(body))
		return
	}

	var overResp OverpassResponse
	if err := json.Unmarshal(body, &overResp); err != nil {
		log.Println("解析 Overpass 回應失敗:", err)
		fmt.Println("Raw body:", string(body))
		return
	}

	fmt.Println("=== Overpass / OpenStreetMap 測試結果 ===")
	fmt.Printf("共拿到 %d 個元素\n", len(overResp.Elements))

	for i, e := range overResp.Elements {
		name := e.Tags["name"]
		if name == "" {
			continue
		}
		fmt.Printf("%d. %s  (lat=%.5f, lon=%.5f)\n", i+1, name, e.Lat, e.Lon)
	}
}
