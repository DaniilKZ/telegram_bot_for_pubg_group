package cron_lunch

import (
	"fmt"
	"log"
	"net/http"

	"tgbot/shared"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	log.Println("[lunch] cron запущен")

	bot, err := shared.NewBot()
	if err != nil {
		log.Printf("[lunch] ❌ бот: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}

	chatID, err := shared.ChatID()
	if err != nil {
		log.Printf("[lunch] ❌ chatID: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}

	quote, author, err := shared.FetchQuote()
	if err != nil {
		log.Printf("[lunch] ❌ цитата: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	log.Printf("[lunch] цитата получена: автор=%q", author)

	text := fmt.Sprintf("🍽 Обеденная цитата:\n\n\u201c%s\u201d\n\n© %s", quote, author)
	if err := shared.Send(bot, chatID, text); err != nil {
		log.Printf("[lunch] ❌ отправка: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}

	log.Println("[lunch] ✅ отправлено")
	fmt.Fprintln(w, "ok")
}
