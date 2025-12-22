package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// getIATACode 使用 Gemini 查詢地點的 IATA 代碼
func getIATACode(c *gin.Context) {
	var req struct {
		Location string `json:"location"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Missing location"})
		return
	}

	ctx := c.Request.Context()
	apiKey := os.Getenv("GEMINI_API_KEY")
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		c.JSON(500, gin.H{"error": "Client error"})
		return
	}
	defer client.Close()

	// 使用輕量模型速度較快
	model := client.GenerativeModel("gemini-2.5-flash-lite")
	model.SetTemperature(0.0) // 溫度設為 0，追求準確與一致性

	//  關鍵 Prompt：要求只回傳代碼
	prompt := fmt.Sprintf(`
    你是一個 IATA 機場代碼查詢 API。
    使用者會輸入一個城市或地點名稱 (可能是中文、英文或有錯字)。
    請回傳該地點最主要的「機場代碼」或「城市代碼」(3個大寫英文字母)。
    
    規則：
    1. 只回傳 3 個大寫字母 (例如: TPE, KIX, NRT, LON)。
    2. 不要包含任何解釋、標點符號或 Markdown 格式。
    3. 如果地點模糊 (例如 "關西")，優先回傳最常用的國際機場 (如 KIX)。
    4. 如果是城市 (如 "東京")，回傳城市代碼 (TYO) 優於特定機場 (NRT)，除非使用者指定機場。
    5. 如果完全無法辨識，回傳 "UNK"。

    使用者輸入: "%s"
    `, req.Location)

	res, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		c.JSON(500, gin.H{"error": "Gemini error: " + err.Error()})
		return
	}

	// 解析回傳結果
	code := "UNK"
	if len(res.Candidates) > 0 {
		for _, part := range res.Candidates[0].Content.Parts {
			if txt, ok := part.(genai.Text); ok {
				// 去除空白與換行
				code = strings.TrimSpace(string(txt))
				// 再次確保只留前3碼 (防止 AI 多話)
				if len(code) > 3 {
					re := regexp.MustCompile(`[A-Z]{3}`)
					found := re.FindString(code)
					if found != "" {
						code = found
					}
				}
			}
		}
	}

	c.JSON(200, gin.H{"code": code})
}
