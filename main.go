package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func loadEnv(filename string) {
	f, err := os.Open(filename)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || line[0] == '#' {
			continue
		}
		parts := splitOnce(line, '=')
		if parts[0] != "" && parts[1] != "" {
			os.Setenv(parts[0], parts[1])
		}
	}
}

func splitOnce(s string, sep byte) [2]string {
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			return [2]string{s[:i], s[i+1:]}
		}
	}
	return [2]string{s, ""}
}

func main() {
	loadEnv(".env")

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN не задан в .env")
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Бот запущен: @%s", bot.Self.UserName)

	go scheduler(bot)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {

		// Нажатие на inline-кнопку
		if update.CallbackQuery != nil {
			cb := update.CallbackQuery
			if cb.Data == "dont_click" {
				// Всплывающее уведомление
				bot.Request(tgbotapi.NewCallback(cb.ID, "😈 Я же сказал не нажимать!"))

				// Меняем текст кнопки
				bot.Request(tgbotapi.NewEditMessageReplyMarkup(
					cb.Message.Chat.ID,
					cb.Message.MessageID,
					tgbotapi.InlineKeyboardMarkup{
						InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
							{tgbotapi.NewInlineKeyboardButtonData("💀 Ты нажал...", "dont_click")},
						},
					},
				))
			}
			continue
		}

		if update.Message == nil {
			continue
		}

		msg := update.Message

		// /test — тестовая отправка утреннего сообщения
		if msg.IsCommand() && msg.Command() == "test" {
			chatIDStr := os.Getenv("CHAT_ID")
			var chatID int64
			fmt.Sscanf(chatIDStr, "%d", &chatID)

			if chatID == 0 {
				log.Printf("❌ CHAT_ID не задан в .env")
			} else {
				testMsg := tgbotapi.NewMessage(chatID, "☀️ Доброе утро чат!")
				if _, err := bot.Send(testMsg); err != nil {
					log.Printf("❌ Ошибка отправки: %v", err)
				} else {
					log.Printf("✅ Сообщение отправлено в чат %d", chatID)
				}
			}
		}

		// /help — приветствие с кнопкой-ловушкой
		if msg.IsCommand() && msg.Command() == "help" {
			username := msg.From.FirstName
			if msg.From.UserName != "" {
				username = "@" + msg.From.UserName
			}

			text := fmt.Sprintf("👋 Привет, %s!\n\n⚠️ Внизу кнопка — НЕ НАЖИМАЙ!", username)

			reply := tgbotapi.NewMessage(msg.Chat.ID, text)
			reply.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("🚫 НЕ НАЖИМАЙ СЮДА !", "dont_click"),
				),
			)
			bot.Send(reply)
		}
	}
}

// scheduler — каждый день в 10:00 по Алматы шлёт «Доброе утро чат!»
func scheduler(bot *tgbotapi.BotAPI) {
	loc, err := time.LoadLocation("Asia/Almaty")
	if err != nil {
		loc = time.FixedZone("UTC+5", 5*60*60)
	}

	chatIDStr := os.Getenv("CHAT_ID")
	if chatIDStr == "" {
		log.Println("CHAT_ID не задан — утренние сообщения отключены. Добавь CHAT_ID в .env")
		return
	}

	var chatID int64
	fmt.Sscanf(chatIDStr, "%d", &chatID)

	for {
		now := time.Now().In(loc)
		next := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, loc)
		if !now.Before(next) {
			next = next.Add(24 * time.Hour)
		}

		log.Printf("Следующее утреннее сообщение через: %v", next.Sub(now))
		time.Sleep(time.Until(next))

		msg := tgbotapi.NewMessage(chatID, "☀️ Доброе утро чат!")
		if _, err := bot.Send(msg); err != nil {
			log.Printf("Ошибка отправки: %v", err)
		} else {
			log.Printf("Утреннее сообщение отправлено в %d", chatID)
		}
	}
}
