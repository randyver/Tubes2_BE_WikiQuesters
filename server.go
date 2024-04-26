package main

import (
	bfs "Tubes2_BE_WikiQuesters/logic"
	ids "Tubes2_BE_WikiQuesters/logic/idsLogic"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

type FormData struct {
	StartPage  string `json:"startPage"`
	TargetPage string `json:"targetPage"`
	Algorithm  string `json:"algorithm"`
}

func main() {

	router := gin.Default()
	router.POST("/api/submit", submitHandler)

	betterRouter := enableCORS(jsonContentTypeMiddleware(router))

	server := &http.Server{
		Addr:    ":8080",
		Handler: betterRouter,
	}

	go func() {
		fmt.Println("Server is listening on port 8080...")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	fmt.Println("Server exiting")
}

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*") // Allow any origin
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Check if the request is for CORS preflight
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Pass down the request to the next middleware (or final handler)
		next.ServeHTTP(w, r)
	})
}

func jsonContentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set JSON Content-Type
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func validateFormData(formData FormData) error {
	// Contoh validasi sederhana
	if formData.StartPage == "" || formData.TargetPage == "" || formData.Algorithm == "" {
		return fmt.Errorf("incomplete form data")
	}
	// Tambahkan validasi lainnya di sini
	return nil
}

func urlToTitle(url string) string {
	// Check if the URL is valid
	if !strings.HasPrefix(url, "https://en.wikipedia.org/wiki/") {
		return ""
	}

	// Extract the part after "wiki/"
	parts := strings.Split(url, "wiki/")
	title := parts[1]

	// Replace underscores with spaces
	title = strings.ReplaceAll(title, "_", " ")

	return title
}

func submitHandler(c *gin.Context) {
	r := (*(*c).Request)
	if r.Method != "POST" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid method"})
		return
	}

	var formData FormData
	err := json.NewDecoder(r.Body).Decode(&formData)
	if err != nil {
		log.Printf("Failed to parse request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse request body"})
		return
	}

	err = validateFormData(formData)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "error algorithm"})
		return
	}

	var result map[string][]string
	var time int64
	var visited int64
	var path_length int
	if formData.Algorithm == "BFS" {
		result, time, visited, path_length = bfs.BfsMultiThread(formData.StartPage, formData.TargetPage)
	} else if formData.Algorithm == "IDS" {
		result, time, visited, path_length = ids.GetIdsResult(formData.StartPage, formData.TargetPage)
	}

	var mapData string = ""

	mapData += "[\n"
	counter := 0
	for key, value := range result {
		mapData += "{\n"
		mapData += "\"" + urlToTitle(key) + "\""
		mapData += ":\n[\n"
		for _, link := range value {
			mapData += "\"" + urlToTitle(link) + "\"\n"
			if link != value[len(value)-1] {
				mapData += ","
			}
		}
		mapData += "]\n}"
		counter += 1
		if counter != len(result) {
			mapData += ","
		}
		mapData += "\n"
	}
	mapData += "]"

	c.JSON(http.StatusOK, gin.H{
		"paths":        mapData,
		"time":         time,
		"path_length":  path_length,
		"visitedCount": visited,
	})
}
