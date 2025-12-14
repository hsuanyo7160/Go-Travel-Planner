# Go-Travel-Planner

一個使用 Go 語言開發的旅遊行程規劃系統

## 專案結構

```
Go-Travel-Planner/
├── backend/        # 後端 API + 前端服務
│   ├── main.go    # 主程式
│   └── go.mod     # Go 模組設定
├── static/        # 前端靜態檔案
│   ├── index.html # 主頁面
│   ├── chat.html  # 聊天介面
│   └── style.css  # 樣式檔
└── start.sh       # 啟動腳本
```

## 狀況檢查

```bash
./status.sh

bash status.sh
```

## 注意事項

必要：
自己的 api key 開在 /backend/.env 下就可以用了
GEMINI_API_KEY = 你的\_gemini_key

非必要：
UNSPLASH_ACCESS_KEY = 你的\_unsplash_access_key (用於顯示圖片)

## 快速開始

### 方法一：使用啟動腳本（推薦）

```bash
go run main.go
```

### 方法二：直接執行

```bash
cd backend
go run main.go
```

### 方法三：使用提供的 Go 版本

```bash
cd backend
/tmp/go/bin/go run main.go
```

## 訪問系統

啟動後開啟瀏覽器訪問：

- **前端介面**: http://localhost:8080/web
- **API 文檔**: http://localhost:8080/api/health

## 功能特色

- ✅ **一體化架構**: 後端同時服務 API 和前端
- ✅ **RESTful API**: 完整的 CRUD 操作
- ✅ **響應式設計**: 適配各種螢幕尺寸
- ✅ **三步驟精靈**: 直觀的行程建立流程
- ✅ **即時互動**: 聊天式行程編輯介面

## API 端點

| 方法   | 路徑             | 說明         |
| ------ | ---------------- | ------------ |
| GET    | `/api/health`    | 健康檢查     |
| GET    | `/api/trips`     | 取得所有行程 |
| GET    | `/api/trips/:id` | 取得特定行程 |
| POST   | `/api/trips`     | 建立新行程   |
| PUT    | `/api/trips/:id` | 更新行程     |
| DELETE | `/api/trips/:id` | 刪除行程     |

## 技術架構

- **後端**: Go + Gin Framework
- **前端**: HTML5 + CSS3 + JavaScript + Bootstrap 5
- **資料存儲**: JSON 檔案
- **通訊協議**: RESTful API

## 系統需求

- Go 1.21 或以上版本
- 瀏覽器支援 HTML5

## 開發建議

1. 實作用戶認證系統
2. 整合地圖服務 (Google Maps)
3. 加入 AI 行程建議功能
4. 匯出功能 (PDF/Excel)
