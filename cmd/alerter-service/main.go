// TODO: просто логирует алерты
package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Simplified webhook structure from Alertmanager
type AlertWebhook struct {
	Alerts []struct {
		Status string `json:"status"`
		Labels struct {
			Alertname string `json:"alertname"`
			Severity  string `json:"severity"`
		} `json:"labels"`
		Annotations struct {
			Summary string `json:"summary"`
		} `json:"annotations"`
	} `json:"alerts"`
}

func alertHandler(w http.ResponseWriter, r *http.Request) {
	var webhook AlertWebhook
	if err := json.NewDecoder(r.Body).Decode(&webhook); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	for _, alert := range webhook.Alerts {
		log.Printf("ALERT %s: %s (%s) - %s",
			alert.Status,
			alert.Labels.Alertname,
			alert.Labels.Severity,
			alert.Annotations.Summary,
		)
	}
	w.WriteHeader(http.StatusOK)
}

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Post("/alert", alertHandler)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})
	log.Println("Alerter service started on :8081")
	log.Fatal(http.ListenAndServe("0.0.0.0:8081", r))
}

//TODO: логирует алерты в телеграм

// package main

// import (
// 	"bytes"
// 	"encoding/json"
// 	"fmt"
// 	"log"
// 	"net/http"
// 	"os"
// )

// type Alert struct {
// 	Status string `json:"status"`
// 	Labels struct {
// 		Alertname string `json:"alertname"`
// 		Severity  string `json:"severity"`
// 	} `json:"labels"`
// 	Annotations struct {
// 		Summary     string `json:"summary"`
// 		Description string `json:"description"`
// 	} `json:"annotations"`
// }

// type AlertWebhook struct {
// 	Alerts []Alert `json:"alerts"`
// }

// // Глобальные переменные для хранения секретов
// var (
// 	telegramBotToken string
// 	telegramChatID   string
// )

// func alertHandler(w http.ResponseWriter, r *http.Request) {
// 	var webhook AlertWebhook
// 	if err := json.NewDecoder(r.Body).Decode(&webhook); err != nil {
// 		http.Error(w, "Bad Request", http.StatusBadRequest)
// 		return
// 	}

// 	// Проходим по всем алертам в вебхуке
// 	for _, alert := range webhook.Alerts {
// 		// Логируем в консоль, как и раньше
// 		log.Printf("🚨 ALERT %s: %s (%s) - %s",
// 			alert.Status,
// 			alert.Labels.Alertname,
// 			alert.Labels.Severity,
// 			alert.Annotations.Summary,
// 		)

// 		// Отправляем уведомление в Telegram
// 		if err := sendTelegramNotification(alert); err != nil {
// 			log.Printf("ERROR: Failed to send Telegram notification: %v", err)
// 		}
// 	}
// 	w.WriteHeader(http.StatusOK)
// }

// // функция для отправки уведомления
// func sendTelegramNotification(alert Alert) error {
// 	// Формируем красивое сообщение
// 	message := fmt.Sprintf(
// 		"**🔥 %s: %s 🔥**\n\n"+
// 			"**Severity:** `%s`\n"+
// 			"**Summary:** %s\n"+
// 			"**Description:** %s",
// 		alert.Status,
// 		alert.Labels.Alertname,
// 		alert.Labels.Severity,
// 		alert.Annotations.Summary,
// 		alert.Annotations.Description,
// 	)

// 	// Формируем тело запроса к Telegram API
// 	requestBody, err := json.Marshal(map[string]string{
// 		"chat_id":    telegramChatID,
// 		"text":       message,
// 		"parse_mode": "Markdown",
// 	})
// 	if err != nil {
// 		return err
// 	}

// 	// Отправляем HTTP-запрос
// 	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", telegramBotToken)
// 	resp, err := http.Post(url, "application/json", bytes.NewBuffer(requestBody))
// 	if err != nil {
// 		return err
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		return fmt.Errorf("telegram API returned non-200 status: %d", resp.StatusCode)
// 	}

// 	return nil
// }

// func main() {
// 	// Читаем секреты из переменных окружения при старте
// 	telegramBotToken = os.Getenv("TELEGRAM_BOT_TOKEN")
// 	telegramChatID = os.Getenv("TELEGRAM_CHAT_ID")

// 	if telegramBotToken == "" || telegramChatID == "" {
// 		log.Fatal("TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID must be set")
// 	}

// 	http.HandleFunc("/alert", alertHandler)
// 	log.Println("Alerter service started on :8080, configured for Telegram notifications.")
// 	log.Fatal(http.ListenAndServe(":8080", nil))
// }
