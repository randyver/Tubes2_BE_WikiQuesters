package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type FormData struct {
	StartPage  string `json:"startPage"`
	TargetPage string `json:"targetPage"`
	Algorithm  string `json:"algorithm"`
}

func validateFormData(formData FormData) error {
	// Contoh validasi sederhana
	if formData.StartPage == "" || formData.TargetPage == "" || formData.Algorithm == "" {
		return fmt.Errorf("incomplete form data")
	}
	// Tambahkan validasi lainnya di sini
	return nil
}

func handleSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var formData FormData
	err := json.NewDecoder(r.Body).Decode(&formData)
	if err != nil {
		log.Printf("Failed to parse request body: %v", err)
		http.Error(w, "Failed to parse request body", http.StatusBadRequest)
		return
	}

	err = validateFormData(formData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response := map[string]string{
		"message": fmt.Sprintf("Received Form Data: %+v", formData),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func main() {
	server := &http.Server{
		Addr: ":8080",
		// Tambahkan middleware dan konfigurasi lainnya di sini
	}

	http.HandleFunc("/api/submit", handleSubmit)

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
