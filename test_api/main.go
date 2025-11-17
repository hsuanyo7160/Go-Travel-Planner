package main

import (
	"fmt"
	"log"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法:")
		fmt.Println("  go run . nominatim   # 測 Nominatim 城市→座標")
		fmt.Println("  go run . otm         # 測 OpenTripMap 附近景點")
		fmt.Println("  go run . meteo       # 測 Open-Meteo 天氣")
		fmt.Println("  go run . gemini      # 測 Gemini AI 內容生成")
		return
	}

	switch os.Args[1] {
	case "nominatim":
		test_nominatim()
	case "otm":
		test_opentripmap()
	case "meteo":
		test_openmeteo()
	case "gemini":
		test_gemini()
	default:
		log.Fatalf("未知指令: %s\n", os.Args[1])
	}
}
