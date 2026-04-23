package handler

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"tgbot/shared"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	log.Println("[morning] cron запущен")

	bot, err := shared.NewBot()
	if err != nil {
		log.Printf("[morning] ❌ бот: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}

	chatID, err := shared.ChatID()
	if err != nil {
		log.Printf("[morning] ❌ chatID: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}

	// День недели по Алматы
	loc := time.FixedZone("UTC+5", 5*60*60)
	now := time.Now().In(loc)
	day := int(now.Weekday())
	_, week := now.ISOWeek()
	variant := week % 4

	text := shared.MorningMessages[day][variant]
	log.Printf("[morning] день=%d вариант=%d текст=%q", day, variant, text[:30])

	if err := shared.Send(bot, chatID, text); err != nil {
		log.Printf("[morning] ❌ отправка: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}

	log.Println("[morning] ✅ отправлено")
	fmt.Fprintln(w, "ok")
}
