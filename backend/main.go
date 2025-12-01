package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	// "sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

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
	MongoID     primitive.ObjectID `bson:"_id,omitempty" json:"-"`
	ID          int                `json:"id"`
	Name        string             `json:"name"`
	Region      string             `json:"region"`
	StartDate   string             `json:"start_date"`
	Days        int                `json:"days"`
	BudgetTWD   int                `json:"budget_twd"`
	People      int                `json:"people"`
	DailyHours  int                `json:"daily_hours"`
	Preferences Preferences        `json:"preferences"`
	Plan        []Day              `json:"plan"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
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

	var updateData Trip
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	updateData.UpdatedAt = time.Now()

	update := bson.M{
		"name":        updateData.Name,
		"region":      updateData.Region,
		"start_date":  updateData.StartDate,
		"days":        updateData.Days,
		"budget_twd":  updateData.BudgetTWD,
		"people":      updateData.People,
		"daily_hours": updateData.DailyHours,
		"preferences": updateData.Preferences,
		"plan":        updateData.Plan,
		"updated_at":  updateData.UpdatedAt,
	}

	_, err = tripsCollection.UpdateOne(
		context.Background(),
		bson.M{"id": id},
		bson.M{"$set": update},
	)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
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
// chatWithGemini 處理帶有上下文的對話
func chatWithGemini(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "JSON 格式錯誤: " + err.Error()})
		return
	}

	ctx := c.Request.Context()

	// 1. 建立 Client
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		c.JSON(500, gin.H{"error": "未設定 GEMINI_API_KEY"})
		return
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		c.JSON(500, gin.H{"error": "無法建立 Gemini Client: " + err.Error()})
		return
	}
	defer client.Close()

	// 2. 設定模型 (關鍵修改區)
	model := client.GenerativeModel("gemini-2.0-flash")

	// [修改 1] 設定更嚴格的 System Instruction，強制它只講人話
	// 這裡我們明確告訴 AI：你是助手，不是資料庫，禁止輸出 JSON
	model.SystemInstruction = genai.NewUserContent(genai.Text(
		"你是一個專業的台灣旅遊規劃助手。請用繁體中文回答，語氣親切。" +
			"請直接以「純文字」或「Markdown」條列式呈現行程，" +
			"**絕對不要**輸出 JSON 格式或程式碼區塊。" +
			"請確保回答完整，不要中斷。",
	))

	// [修改 2] 增加回應長度上限 (預設有時太短，設為 8192 確保長行程能寫完)
	model.SetMaxOutputTokens(8192)

	// (可選) 調整溫度，讓回答穩定一點
	model.SetTemperature(0.7)

	// 3. 建立 Chat Session 並填入歷史紀錄
	cs := model.StartChat()

	if len(req.History) > 0 {
		var chatHistory []*genai.Content
		for _, h := range req.History {
			// [重要] 這裡建議做一個簡單的過濾
			// 如果歷史紀錄裡有包含 "{" 這種看起來像 JSON 的，最好不要傳給模型
			// 或者確保前端傳來的 history 只是單純的對話文字

			role := "user"
			if h.Role == "model" || h.Role == "assistant" {
				role = "model"
			}

			chatHistory = append(chatHistory, &genai.Content{
				Role: role,
				Parts: []genai.Part{
					genai.Text(h.Text),
				},
			})
		}
		cs.History = chatHistory
	}

	// 4. 發送這次的訊息
	res, err := cs.SendMessage(ctx, genai.Text(req.Message))
	if err != nil {
		c.JSON(500, gin.H{"error": "Gemini 回應錯誤: " + err.Error()})
		return
	}

	// 5. 組合回應文字
	var responseText string
	if len(res.Candidates) > 0 {
		for _, part := range res.Candidates[0].Content.Parts {
			if txt, ok := part.(genai.Text); ok {
				responseText += string(txt)
			}
		}
	}

	c.JSON(200, gin.H{
		"reply": responseText,
	})
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
		modelName = "gemini-2.0-flash" // 建議使用目前穩定的版本
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
