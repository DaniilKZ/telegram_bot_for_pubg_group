package handler

import (
	"fmt"
	"log"
	"net/http"
	"tgbot/api/_shared"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	log.Println("[evening] cron запущен")

	bot, err := _shared.NewBot()
	if err != nil {
		log.Printf("[evening] ❌ бот: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}

	chatID, err := _shared.ChatID()
	if err != nil {
		log.Printf("[evening] ❌ chatID: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}

	text := "🌙 День подходит к концу!\n\nНадеемся, он был продуктивным и наполненным 😊\nОтдыхайте, завтра будет новый день — желаем всем спокойного вечера и хорошего отдыха 🌟"
	if err := _shared.Send(bot, chatID, text); err != nil {
		log.Printf("[evening] ❌ отправка: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}

	log.Println("[evening] ✅ отправлено")
	fmt.Fprintln(w, "ok")
}
