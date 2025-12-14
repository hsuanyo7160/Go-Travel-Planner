package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	// "sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	//"google.golang.org/genai"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ========== 資料模型 ==========
type Trip struct {
	MongoID primitive.ObjectID `bson:"_id,omitempty" json:"-"`

	// ▼▼▼ 修改這裡：加上 bson:"..." 以確保資料庫欄位名稱統一 ▼▼▼
	ID          int         `json:"id" bson:"id"`
	Name        string      `json:"name" bson:"name"`
	Region      string      `json:"region" bson:"region"`
	StartDate   string      `json:"start_date" bson:"start_date"`
	Days        int         `json:"days" bson:"days"`
	BudgetTWD   int         `json:"budget_twd" bson:"budget_twd"`
	People      int         `json:"people" bson:"people"`
	DailyHours  int         `json:"daily_hours" bson:"daily_hours"`
	Preferences Preferences `json:"preferences" bson:"preferences"`
	Plan        []Day       `json:"plan" bson:"plan"`
	CreatedAt   time.Time   `json:"created_at" bson:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at" bson:"updated_at"`
}

type Preferences struct {
	Pace      string   `json:"pace"`
	Types     []string `json:"types"`
	Transport []string `json:"transport"`
	Dining    []string `json:"dining"`
}

type Day struct {
	DayIndex int    `json:"day_index"`
	Date     string `json:"date"`
	Items    []Item `json:"items"`
}

type Item struct {
	ID          string  `json:"id"`
	Time        string  `json:"time"`
	DurationMin int     `json:"duration_min"`
	Title       string  `json:"title"`
	Address     string  `json:"address"`
	Lat         float64 `json:"lat,omitempty"`
	Lng         float64 `json:"lng,omitempty"`
	Link        string  `json:"link"`
	Note        string  `json:"note"`
}

// ChatRequest 前端傳來的請求格式
type ChatRequest struct {
	Message string     `json:"message"` // 使用者這次說的話
	History []ChatPart `json:"history"` // 過去的對話歷史 (可選)
}

// ChatPart 對話歷史的單一則訊息
type ChatPart struct {
	Role string `json:"role"` // "user" (使用者) 或 "model" (AI)
	Text string `json:"text"` // 訊息內容
}

// ========== MongoDB ==========
var mongoClient *mongo.Client
var tripsCollection *mongo.Collection

func initMongo() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatalf("MongoDB connect error: %v", err)
	}

	// 可以決定 db / collection 名稱
	mongoClient = client
	tripsCollection = client.Database("go_travel").Collection("trips")

	log.Println("MongoDB connected")
}

// ========== 主程式 ==========
func main() {
	// 載入 .env 檔案
	godotenv.Load()

	// 連線 MongoDB
	initMongo()

	// 設定 Gin
	r := gin.Default()

	// CORS 設定 - 允許前端跨域請求
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:8080", "*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// 靜態檔案服務 - 使用原本的 /static 資料夾
	r.Static("/web", "../static")
	r.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/web/")
	})

	// API 路由
	api := r.Group("/api")
	{
		// 行程相關
		api.GET("/trips", getTrips)
		api.GET("/trips/:id", getTrip)
		api.POST("/trips", createTrip)
		api.PUT("/trips/:id", updateTrip)
		api.DELETE("/trips/:id", deleteTrip)

		// Gemini 相關 (確保這裡每一行指令的網址都不一樣)
		api.POST("/gemini", callGemini) // 一般問答
		api.POST("/gemini/save", saveGeminiToFile)
		api.GET("/gemini/response", getGeminiResponse)

		// 注意：這裡原本可能有重複的 api.POST("/gemini", callGemini)，請刪除它！

		api.POST("/gemini/chat", chatWithGemini) // 對話模式

		// 健康檢查
		api.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"status": "ok",
				"time":   time.Now(),
			})
		})

		// Unsplash image proxy/search
		api.GET("/unsplash", unsplashHandler)
		// IATA code 查詢
		api.POST("/iata", getIATACode)
	}
	// 啟動伺服器
	port := ":8080"
	log.Printf("Server running on http://localhost%s", port)
	log.Printf("Frontend: http://localhost%s/web", port)
	log.Printf("API: http://localhost%s/api", port)
	if err := r.Run(port); err != nil {
		log.Fatal(err)
	}
}

// ========== API Handlers ==========

func getTrips(c *gin.Context) {
	cursor, err := tripsCollection.Find(context.Background(), bson.M{})
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(context.Background())

	var tripList []Trip
	for cursor.Next(context.Background()) {
		var t Trip
		if err := cursor.Decode(&t); err == nil {
			tripList = append(tripList, t)
		}
	}

	c.JSON(200, tripList)
}

func getTrip(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid ID"})
		return
	}

	var trip Trip
	err = tripsCollection.FindOne(context.Background(), bson.M{"id": id}).Decode(&trip)
	if err != nil {
		c.JSON(404, gin.H{"error": "Trip not found"})
		return
	}

	c.JSON(200, trip)
}

func createTrip(c *gin.Context) {
	var trip Trip
	if err := c.ShouldBindJSON(&trip); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	trip.ID = int(time.Now().Unix())
	trip.CreatedAt = time.Now()
	trip.UpdatedAt = time.Now()

	if trip.Plan == nil || len(trip.Plan) == 0 {
		trip.Plan = expandDays(trip.StartDate, trip.Days)
	}

	_, err := tripsCollection.InsertOne(context.Background(), trip)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(201, trip)
}

func updateTrip(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid ID"})
		return
	}

	// 1. 先用 map 接收前端傳來的資料，這樣才能知道前端「到底傳了哪些欄位」
	var rawMap map[string]interface{}
	if err := c.ShouldBindJSON(&rawMap); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// 2. 準備要更新的 bson map
	update := bson.M{
		"updated_at": time.Now(),
	}

	// 3. 逐一檢查欄位，有傳才更新
	if v, ok := rawMap["name"]; ok {
		update["name"] = v
	}
	if v, ok := rawMap["region"]; ok {
		update["region"] = v
	}
	if v, ok := rawMap["start_date"]; ok {
		update["start_date"] = v
	}
	if v, ok := rawMap["days"]; ok {
		update["days"] = v
	}
	if v, ok := rawMap["budget_twd"]; ok {
		update["budget_twd"] = v
	}
	if v, ok := rawMap["people"]; ok {
		update["people"] = v
	}
	if v, ok := rawMap["daily_hours"]; ok {
		update["daily_hours"] = v
	}
	if v, ok := rawMap["preferences"]; ok {
		update["preferences"] = v
	}

	// ⚠️ 關鍵：只有當前端真的傳了 "plan" 欄位時，才去更新它
	// 如果前端沒傳 (因為是微調模式)，這裡就不會把 plan 覆蓋掉
	if v, ok := rawMap["plan"]; ok {
		update["plan"] = v
	}

	// 4. 執行更新
	result, err := tripsCollection.UpdateOne(
		context.Background(),
		bson.M{"id": id},
		bson.M{"$set": update},
	)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	if result.MatchedCount == 0 {
		c.JSON(404, gin.H{"error": "Trip not found"})
		return
	}

	c.JSON(200, gin.H{"message": "Trip updated"})
}

func deleteTrip(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid ID"})
		return
	}

	_, err = tripsCollection.DeleteOne(context.Background(), bson.M{"id": id})
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Trip deleted"})
}

// chatWithGemini 處理帶有上下文的對話
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
		// 🛑 重點：這裡會印出真正的錯誤原因！
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

// ========== Unsplash helper (simple proxy + cache) ==========
var unsplashCache = struct {
	mu sync.Mutex
	m  map[string]string
}{m: make(map[string]string)}

func unsplashHandler(c *gin.Context) {
	q := c.Query("query")
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing query"})
		return
	}

	// simple cache key
	key := strings.ToLower(strings.TrimSpace(q))
	unsplashCache.mu.Lock()
	if v, ok := unsplashCache.m[key]; ok {
		unsplashCache.mu.Unlock()
		c.JSON(200, gin.H{"url": v})
		return
	}
	unsplashCache.mu.Unlock()

	accessKey := os.Getenv("UNSPLASH_ACCESS_KEY")
	if accessKey == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "UNSPLASH_ACCESS_KEY not set"})
		return
	}

	api := fmt.Sprintf("https://api.unsplash.com/search/photos?query=%s&per_page=1", url.QueryEscape(q))
	req, err := http.NewRequest("GET", api, nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	req.Header.Set("Authorization", "Client-ID "+accessKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	var result struct {
		Results []struct {
			Urls map[string]string `json:"urls"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid response from unsplash"})
		return
	}

	if len(result.Results) > 0 {
		url := result.Results[0].Urls["regular"]
		if url == "" {
			url = result.Results[0].Urls["small"]
		}
		if url != "" {
			unsplashCache.mu.Lock()
			unsplashCache.m[key] = url
			unsplashCache.mu.Unlock()
			c.JSON(200, gin.H{"url": url})
			return
		}
	}

	c.JSON(200, gin.H{"url": ""})
}

// ========== 輔助函數 ==========

func expandDays(startDate string, days int) []Day {
	result := make([]Day, days)
	start, _ := time.Parse("2006-01-02", startDate)

	for i := 0; i < days; i++ {
		date := start.AddDate(0, 0, i)
		result[i] = Day{
			DayIndex: i + 1,
			Date:     date.Format("2006-01-02"),
			Items:    []Item{},
		}
	}

	return result
}

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

	// 💡 關鍵 Prompt：要求只回傳代碼
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
