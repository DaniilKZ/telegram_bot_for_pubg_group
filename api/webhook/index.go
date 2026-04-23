package webhook

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"tgbot/shared"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusOK)
		return
	}

	bot, err := shared.NewBot()
	if err != nil {
		log.Printf("[webhook] ❌ бот: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}

	var update tgbotapi.Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		log.Printf("[webhook] ❌ decode: %v", err)
		http.Error(w, err.Error(), 400)
		return
	}

	// Inline-кнопка
	if update.CallbackQuery != nil {
		cb := update.CallbackQuery
		if cb.Data == "dont_click" {
			bot.Request(tgbotapi.NewCallback(cb.ID, "😈 Я же сказал не нажимать!"))
			bot.Request(tgbotapi.NewEditMessageReplyMarkup(
				cb.Message.Chat.ID, cb.Message.MessageID,
				tgbotapi.InlineKeyboardMarkup{
					InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
						{tgbotapi.NewInlineKeyboardButtonData("💀 Ты нажал...", "dont_click")},
					},
				},
			))
		}
		fmt.Fprintln(w, "ok")
		return
	}

	if update.Message == nil {
		fmt.Fprintln(w, "ok")
		return
	}

	msg := update.Message
	chatID := msg.Chat.ID
	log.Printf("[webhook] @%s chatID=%d: %q", msg.From.UserName, chatID, msg.Text)

	// /test
	if msg.IsCommand() && msg.Command() == "test" {
		targetID, err := shared.ChatID()
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ CHAT_ID не задан в env"))
		} else {
			loc := time.FixedZone("UTC+5", 5*60*60)
			now := time.Now().In(loc)
			day := int(now.Weekday())
			_, week := now.ISOWeek()

			shared.Send(bot, targetID, shared.MorningMessages[day][week%4])

			quote, author, err := shared.FetchQuote()
			if err == nil {
				shared.Send(bot, targetID, fmt.Sprintf("🍽 Обеденная цитата:\n\n\u201c%s\u201d\n\n© %s", quote, author))
			}

			shared.Send(bot, targetID, "🌙 День подходит к концу!\n\nНадеемся, он был продуктивным и наполненным 😊\nОтдыхайте, завтра будет новый день — желаем всем спокойного вечера и хорошего отдыха 🌟")
			bot.Send(tgbotapi.NewMessage(chatID, "✅ Все три сообщения отправлены"))
		}
	}

	// /quote
	if msg.IsCommand() && msg.Command() == "quote" {
		quote, author, err := shared.FetchQuote()
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Не удалось получить цитату"))
		} else {
			bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("\u201c%s\u201d\n\n© %s", quote, author)))
		}
	}

	// /help
	if msg.IsCommand() && msg.Command() == "help" {
		username := msg.From.FirstName
		if msg.From.UserName != "" {
			username = "@" + msg.From.UserName
		}
		reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("👋 Привет, %s!\n\n⚠️ Внизу кнопка — НЕ НАЖИМАЙ!", username))
		reply.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🚫 НЕ НАЖИМАЙ СЮДА !", "dont_click"),
			),
		)
		bot.Send(reply)
	}

	fmt.Fprintln(w, "ok")
}
