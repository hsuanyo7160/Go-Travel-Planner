package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// chatWithGemini 處理帶有上下文的對話 (Debug 版)
func chatWithGemini(c *gin.Context) {
	fmt.Println("🚀 收到對話請求...") // Debug Log

	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fmt.Println("❌ JSON 解析失敗:", err)
		c.JSON(400, gin.H{"error": "JSON 格式錯誤: " + err.Error()})
		return
	}

	ctx := c.Request.Context()

	// 你的 API Key (確認已填入)
	apiKey := os.Getenv("GEMINI_API_KEY")

	fmt.Println("🔑 使用 API Key:", apiKey[:10]+"...") // 只印前10碼確認有讀到

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		fmt.Println("❌ 無法建立 Client:", err)
		c.JSON(500, gin.H{"error": "無法建立 Gemini Client: " + err.Error()})
		return
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-2.5-flash-lite")
	model.SystemInstruction = genai.NewUserContent(genai.Text("你是一個專業導遊。"))
	model.SetMaxOutputTokens(8192)
	model.SetTemperature(0.7)

	cs := model.StartChat()

	// 處理歷史紀錄
	if len(req.History) > 0 {
		fmt.Printf("📚 載入歷史紀錄: %d 則\n", len(req.History))
		var chatHistory []*genai.Content
		for _, h := range req.History {
			role := "user"
			if h.Role == "model" || h.Role == "assistant" {
				role = "model"
			}
			chatHistory = append(chatHistory, &genai.Content{
				Role:  role,
				Parts: []genai.Part{genai.Text(h.Text)},
			})
		}
		cs.History = chatHistory
	}

	fmt.Println("📤 正在發送訊息給 Google...")

	// 發送請求
	res, err := cs.SendMessage(ctx, genai.Text(req.Message))
	if err != nil {
		fmt.Println("❌ Gemini API 錯誤:", err)
		c.JSON(500, gin.H{"error": "Gemini API 錯誤: " + err.Error()})
		return
	}

	fmt.Println("✅ 收到 Gemini 回應！")

	var responseText string
	if len(res.Candidates) > 0 {
		for _, part := range res.Candidates[0].Content.Parts {
			if txt, ok := part.(genai.Text); ok {
				responseText += string(txt)
			}
		}
	}

	c.JSON(200, gin.H{"reply": responseText})
}

// ====== Gemini 呼叫 (單次) ======
func callGemini(c *gin.Context) {
	var req struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()

	// 1. 建立 Client (同樣建議改用環境變數)
	apiKey := os.Getenv("GEMINI_API_KEY")
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		c.JSON(500, gin.H{"error": "Client error: " + err.Error()})
		return
	}
	defer client.Close()

	// 2. 設定模型
	modelName := req.Model
	if modelName == "" {
		modelName = "gemini-2.5-flash-lite" // 建議使用目前穩定的版本
	}
	model := client.GenerativeModel(modelName)

	// 3. 發送請求 (新版 SDK 語法)
	res, err := model.GenerateContent(ctx, genai.Text(req.Prompt))
	if err != nil {
		c.JSON(500, gin.H{"error": "Generate error: " + err.Error()})
		return
	}

	// 4. 解析回應
	var responseText string
	if len(res.Candidates) > 0 {
		for _, part := range res.Candidates[0].Content.Parts {
			if txt, ok := part.(genai.Text); ok {
				responseText += string(txt)
			}
		}
	}

	c.JSON(200, gin.H{"text": responseText})
}

// saveGeminiToFile 將收到的文字儲存為 data 目錄下的檔案
func saveGeminiToFile(c *gin.Context) {
	var req struct {
		Filename string `json:"filename"`
		Name     string `json:"name"`
		Text     string `json:"text"`
		Format   string `json:"format"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	name := req.Filename
	if name == "" {
		name = fmt.Sprintf("gemini_%d", time.Now().Unix())
	}

	// 只保留安全字元
	re := regexp.MustCompile(`[^A-Za-z0-9._-]`)
	name = re.ReplaceAllString(name, "_")

	ext := ".txt"
	if strings.ToLower(req.Format) == "json" {
		ext = ".json"
	}

	// 若為 JSON 格式，固定檔名為 response.json（寫入 data/response.json）
	if strings.ToLower(req.Format) == "json" {
		name = "response" + ext
	} else {
		if !strings.HasSuffix(name, ext) {
			name = name + ext
		}
	}

	dest := filepath.Join("../data", name)

	// 若請求要求 JSON 格式，將回應切段並 append 到目標檔案的 response 陣列
	if strings.ToLower(req.Format) == "json" {
		// 不切段：將整段回覆視為 single element，trim 後 append（若為空則不 append）
		var parts []string
		if t := strings.TrimSpace(req.Text); t != "" {
			parts = []string{t}
		} else {
			parts = []string{}
		}

		type OutFile struct {
			Name     string   `json:"name"`
			Response []string `json:"response"`
		}

		var out OutFile
		// 若檔案已存在，讀取並合併
		if b, err := os.ReadFile(dest); err == nil {
			if err := json.Unmarshal(b, &out); err != nil {
				// 若既有檔案不是期望格式，覆寫為新格式
				out = OutFile{Name: req.Name, Response: []string{}}
			}
		} else {
			// 檔案不存在，建立新結構
			out = OutFile{Name: req.Name, Response: []string{}}
		}

		// 若 out.Name 為空，填入 req.Name
		if out.Name == "" {
			out.Name = req.Name
		}

		// Append parts
		out.Response = append(out.Response, parts...)

		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		// 寫回檔案（覆蓋）
		if err := os.WriteFile(dest, data, 0644); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{"path": dest})
		return
	}

	// 否則當作純文字寫入
	if err := os.WriteFile(dest, []byte(req.Text), 0644); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"path": dest})
}

// getGeminiResponse 讀取 data/response.json 並回傳 JSON 結構
func getGeminiResponse(c *gin.Context) {
	path := filepath.Join("../data", "response.json")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// 回傳空的結構
			c.JSON(200, gin.H{"name": "", "response": []string{}})
			return
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	var out interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		// 若檔案不是合法 JSON，回傳原始文字
		c.JSON(200, gin.H{"raw": string(b)})
		return
	}
	c.JSON(200, out)
}
