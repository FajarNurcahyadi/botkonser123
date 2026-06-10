package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	BotToken        = "8980353220:AAFdNAZutNsHVb8m7-J_l3_DFC0Tp5Ne4Jg"
	ChatID          = int64(8072635161)
	TargetURL       = "https://webkonser.com/api/tickets/1" // ganti sesuai endpoint tiket
	FlareSolverrURL = "http://localhost:8191/v1"
	CheckInterval   = 1 * time.Second

	// Auto buy config
	BuyURL      = "https://webkonser.com/api/tickets/buy" // ganti sesuai endpoint checkout
	UserSession = "your-session-cookie-here"              // isi cookie session lo setelah login manual
)

// ===== STRUCTS =====

type Product struct {
	Title string `json:"title"`
	Stock int    `json:"stock"`
	ID    int    `json:"id"`
}

type TelegramMessage struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

// Flaresolverr request/response
type FlareSolverrRequest struct {
	CMD        string `json:"cmd"`
	URL        string `json:"url"`
	MaxTimeout int    `json:"maxTimeout"`
	Session    string `json:"session,omitempty"`
}

type FlareSolverrResponse struct {
	Status   string `json:"status"`
	Solution struct {
		Response string            `json:"response"`
		Cookies  []FlareCookie     `json:"cookies"`
		Headers  map[string]string `json:"headers"`
		URL      string            `json:"url"`
	} `json:"solution"`
}

type FlareCookie struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Auto buy payload — sesuaikan dengan form checkout website target
type BuyPayload struct {
	TicketID int    `json:"ticket_id"`
	Quantity int    `json:"quantity"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
}

// ===== MAIN =====

func main() {
	log.Println("Bot war ticket started...")
	log.Println("Flaresolverr:", FlareSolverrURL)
	log.Println("Target:", TargetURL)

	ticker := time.NewTicker(CheckInterval)
	defer ticker.Stop()

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		for range ticker.C {
			go checkKetersediaan()
		}
	}()

	wg.Wait()
}

// ===== CEK STOK =====

func checkKetersediaan() {
	body, err := fetchViaFlaresolverr(TargetURL)
	if err != nil {
		log.Printf("Error fetch: %v", err)
		return
	}

	var product Product
	if err := json.Unmarshal([]byte(body), &product); err != nil {
		log.Printf("Error parse JSON: %v", err)
		return
	}

	if product.Stock > 0 {
		msg := fmt.Sprintf("🎫 Tiket '%s' TERSEDIA! Stock: %d\nSedang auto buy...", product.Title, product.Stock)
		log.Println(msg)
		go sendTelegramMessage(msg)
		go autoBuy(product)
	} else {
		log.Printf("Tiket '%s' belum tersedia.", product.Title)
	}
}

// ===== FETCH VIA FLARESOLVERR =====

func fetchViaFlaresolverr(url string) (string, error) {
	reqBody := FlareSolverrRequest{
		CMD:        "request.get",
		URL:        url,
		MaxTimeout: 60000,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal error: %v", err)
	}

	resp, err := http.Post(FlareSolverrURL, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("flaresolverr error: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read error: %v", err)
	}

	var fsResp FlareSolverrResponse
	if err := json.Unmarshal(respBody, &fsResp); err != nil {
		return "", fmt.Errorf("parse flaresolverr response error: %v", err)
	}

	if fsResp.Status != "ok" {
		return "", fmt.Errorf("flaresolverr status: %s", fsResp.Status)
	}

	return fsResp.Solution.Response, nil
}

// ===== AUTO BUY =====

func autoBuy(product Product) {
	payload := BuyPayload{
		TicketID: product.ID,
		Quantity: 1,
		Name:     "Nama Lo",      // ganti
		Email:    "email@lo.com", // ganti
		Phone:    "08xxxxxxxxxx", // ganti
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Auto buy marshal error: %v", err)
		return
	}

	req, err := http.NewRequest("POST", BuyURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		log.Printf("Auto buy request error: %v", err)
		return
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", UserSession) // session dari login manual
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", TargetURL)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Auto buy error: %v", err)
		sendTelegramMessage("❌ Auto buy GAGAL: " + err.Error())
		return
	}
	defer resp.Body.Close()

	respBody, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK || resp.StatusCode == 201 {
		msg := fmt.Sprintf("✅ Auto buy BERHASIL!\nTiket: %s\nResponse: %s", product.Title, string(respBody))
		log.Println(msg)
		sendTelegramMessage(msg)
	} else {
		msg := fmt.Sprintf("❌ Auto buy GAGAL! Status: %d\nResponse: %s", resp.StatusCode, string(respBody))
		log.Println(msg)
		sendTelegramMessage(msg)
	}
}

// ===== TELEGRAM =====

func sendTelegramMessage(message string) {
	urlTelegramBot := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", BotToken)

	payload := TelegramMessage{
		ChatID: ChatID,
		Text:   message,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Telegram marshal error: %v", err)
		return
	}

	resp, err := http.Post(urlTelegramBot, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		log.Printf("Telegram error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Printf("Telegram API error: status %d, body: %s", resp.StatusCode, string(body))
		return
	}
	log.Printf("Telegram sent: %s", message)
}
