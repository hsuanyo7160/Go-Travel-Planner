package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

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
