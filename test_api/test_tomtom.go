package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
)

// 換成 TomTom Routing API 的回傳格式（只抓 summary 就好）
type TomTomRouteResponse struct {
	Routes []struct {
		Summary struct {
			LengthInMeters      int    `json:"lengthInMeters"`
			TravelTimeInSeconds int    `json:"travelTimeInSeconds"`
			DepartureTime       string `json:"departureTime"`
			ArrivalTime         string `json:"arrivalTime"`
		} `json:"summary"`
	} `json:"routes"`
}

// 寫死一段路線：台北 101 → 台北車站（你可以自己改座標）
func test_tomtom() {
	// TODO: 換成你自己的 TomTom API Key
	const apiKey = "J9bqlHe65r8BTsyBc5OIvXaADcssOnUY"

	// 起終點座標（字串就好，方便跟你原本模板對齊）
	startLat := "25.033968" // 台北 101 附近
	startLon := "121.564468"
	endLat := "25.047675" // 台北車站附近
	endLon := "121.517055"

	// TomTom Routing API:
	// GET https://api.tomtom.com/routing/1/calculateRoute/{startLat,startLon}:{endLat,endLon}/json?key=...
	locPath := fmt.Sprintf("%s,%s:%s,%s", startLat, startLon, endLat, endLon)
	baseURL := "https://api.tomtom.com/routing/1/calculateRoute/" + url.PathEscape(locPath) + "/json"

	params := url.Values{}
	params.Set("key", apiKey)
	params.Set("routeType", "fastest") // 最快路線
	params.Set("traffic", "true")      // 考慮即時交通
	params.Set("travelMode", "car")    // car / pedestrian / truck ...

	reqURL := baseURL + "?" + params.Encode()

	resp, err := http.Get(reqURL)
	if err != nil {
		log.Fatal("呼叫 TomTom 失敗:", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("TomTom 回傳非 200，status=%s\n", resp.Status)
	}

	var ttResp TomTomRouteResponse
	if err := json.NewDecoder(resp.Body).Decode(&ttResp); err != nil {
		log.Fatal("解析 TomTom 回應失敗:", err)
	}

	if len(ttResp.Routes) == 0 {
		log.Fatal("TomTom 沒有回傳任何路線")
	}

	route := ttResp.Routes[0]
	dist := route.Summary.LengthInMeters
	sec := route.Summary.TravelTimeInSeconds

	fmt.Println("=== TomTom Routing 測試結果 ===")
	fmt.Printf("起點：(%s, %s)\n", startLat, startLon)
	fmt.Printf("終點：(%s, %s)\n", endLat, endLon)
	fmt.Printf("距離：%d 公尺\n", dist)
	fmt.Printf("預估時間：%d 秒（約 %.1f 分鐘）\n", sec, float64(sec)/60.0)
	fmt.Printf("出發時間：%s\n", route.Summary.DepartureTime)
	fmt.Printf("抵達時間：%s\n", route.Summary.ArrivalTime)
}
