package cron_evening

import (
	"fmt"
	"log"
	"net/http"

	"tgbot/shared"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	log.Println("[evening] cron запущен")

	bot, err := shared.NewBot()
	if err != nil {
		log.Printf("[evening] ❌ бот: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}

	chatID, err := shared.ChatID()
	if err != nil {
		log.Printf("[evening] ❌ chatID: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}

	text := "🌙 День подходит к концу!\n\nНадеемся, он был продуктивным и наполненным 😊\nОтдыхайте, завтра будет новый день — желаем всем спокойного вечера и хорошего отдыха 🌟"
	if err := shared.Send(bot, chatID, text); err != nil {
		log.Printf("[evening] ❌ отправка: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}

	log.Println("[evening] ✅ отправлено")
	fmt.Fprintln(w, "ok")
}
