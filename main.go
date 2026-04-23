package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// -------------------------------------------------------
// Массив утренних сообщений [7 дней][4 варианта]
// -------------------------------------------------------

var morningMessages = [7][4]string{
	// 0 — Воскресенье
	{
		"🧘 Воскресенье — баланс\n\nПроанализируйте неделю и наметьте приоритеты.\nСохраняйте лёгкость — перегруз не нужен.\nПодготовьте спокойный старт новой недели. ✨",
		"☀️ Доброе воскресное утро!\nПусть день будет спокойным и восстановительным.",
		"🌸 Привет!\nПодведите итоги и мягко настройтесь на новую неделю.",
		"✨ Счастливого воскресенья!\nЗавершите выходные в спокойном ритме и с ясным планом.",
	},
	// 1 — Понедельник
	{
		"📅 Понедельник — старт новых возможностей\n\nСегодня — чистый лист. Поставьте цель, продумайте план и сделайте первый шаг.\nНе ждите идеального момента — он уже начался.\nВерьте в себя: у вас есть всё для результата. 💪",
		"☀️ Доброе утро! Пусть начало недели будет лёгким и продуктивным.\nПонедельник — отличный день, чтобы поставить новые цели.\nУдачи и уверенного старта!",
		"✨ С началом недели!\nПусть каждый день приносит что-то полезное и приближает к цели.\nДействуйте последовательно — результат накопится.",
		"👋 Привет! Понедельник — точка запуска.\nСформулируйте приоритеты и начните с малого шага.\nСтабильность важнее резких рывков.",
	},
	// 2 — Вторник
	{
		"🚀 Вторник — набираем обороты\n\nВы уже в движении.\nУглубитесь в задачи и усилите фокус.\nОшибки — это данные для корректировки курса. 🎯",
		"☀️ Доброе утро!\nНеделя набирает ход — пусть вторник принесёт приятные сюрпризы и стабильный прогресс.",
		"💼 Привет!\nВо вторник чувствуется рабочий ритм.\nСохраняйте темп и доводите начатое до результата.",
		"👋 Здравствуйте!\nВторник — время системной работы.\nМалые шаги ежедневно дают заметный итог.",
	},
	// 3 — Среда
	{
		"🌿 Среда — середина пути\n\nОцените достигнутое.\nДаже небольшой прогресс имеет значение.\nСделайте паузу при необходимости и продолжайте движение. 🔄",
		"💡 Доброе утро!\nСреда подходит для новых идей и свежих решений.\nИспользуйте накопленный опыт недели.",
		"👋 Привет!\nПоловина недели позади.\nСфокусируйтесь на приоритетах и завершении ключевых задач.",
		"🌤 Доброго дня!\nНебольшой перерыв, восстановление энергии и снова в работу.\nБаланс ускоряет результат.",
	},
	// 4 — Четверг
	{
		"⚡ Четверг — ускорение\n\nФинишная прямая близко.\nЗакрывайте задачи и усиливайте результат.\nПеред завершением часто происходят прорывы. 🔥",
		"☀️ Доброе утро!\nДо выходных рукой подать.\nПусть четверг принесёт конкретные результаты.",
		"📊 Привет!\nПодведите промежуточные итоги и скорректируйте план.\nСфокусируйтесь на главном.",
		"👋 Здравствуйте!\nЧетверг — время доводить начатое до логического завершения.",
	},
	// 5 — Пятница
	{
		"🎉 Пятница — подведение итогов\n\nНеделя почти завершена.\nОцените достижения и зафиксируйте результат.\nГордитесь проделанной работой. ✔️",
		"😄 Привет!\nПятница — день с особой атмосферой.\nПусть он пройдёт легко и продуктивно.",
		"🌇 Доброго дня!\nЗакройте ключевые задачи и подготовьте лёгкий план на следующую неделю.",
		"🥳 Ура, пятница!\nЗафиксируйте прогресс и переключитесь на восстановление.",
	},
	// 6 — Суббота
	{
		"🛌 Суббота — восстановление\n\nОтдых — это ресурс.\nОтключитесь от задач и восстановите энергию.\nЭто инвестиция в продуктивность. 🌿",
		"☀️ Доброе субботнее утро!\nПроведите день так, как действительно хочется.",
		"🎈 Привет!\nСуббота — время для себя, близких и простых радостей.",
		"🌿 Счастливой субботы!\nОтдыхайте и наполняйтесь энергией без спешки.",
	},
}

// -------------------------------------------------------
// Helpers
// -------------------------------------------------------

func getBot() (*tgbotapi.BotAPI, error) {
	return tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_BOT_TOKEN"))
}

func getChatID() (int64, error) {
	var id int64
	_, err := fmt.Sscanf(os.Getenv("CHAT_ID"), "%d", &id)
	if id == 0 {
		return 0, fmt.Errorf("CHAT_ID не задан")
	}
	return id, err
}

func sendText(bot *tgbotapi.BotAPI, chatID int64, text string) error {
	_, err := bot.Send(tgbotapi.NewMessage(chatID, text))
	return err
}

func fetchQuote() (string, string, error) {
	resp, err := http.Get("https://api.forismatic.com/api/1.0/?method=getQuote&format=json&lang=ru")
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	var result struct {
		QuoteText   string `json:"quoteText"`
		QuoteAuthor string `json:"quoteAuthor"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", err
	}
	if result.QuoteAuthor == "" {
		result.QuoteAuthor = "Неизвестный автор"
	}
	return result.QuoteText, result.QuoteAuthor, nil
}

// -------------------------------------------------------
// Handlers
// -------------------------------------------------------

func handleMorning(w http.ResponseWriter, r *http.Request) {
	log.Println("[morning] cron вызван")
	bot, err := getBot()
	if err != nil {
		log.Printf("[morning] ❌ бот: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	chatID, err := getChatID()
	if err != nil {
		log.Printf("[morning] ❌ chatID: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	loc := time.FixedZone("UTC+5", 5*60*60)
	now := time.Now().In(loc)
	day := int(now.Weekday())
	_, week := now.ISOWeek()
	text := morningMessages[day][week%4]
	if err := sendText(bot, chatID, text); err != nil {
		log.Printf("[morning] ❌ отправка: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	log.Printf("[morning] ✅ день=%d вариант=%d", day, week%4)
	fmt.Fprintln(w, "ok")
}

func handleLunch(w http.ResponseWriter, r *http.Request) {
	log.Println("[lunch] cron вызван")
	bot, err := getBot()
	if err != nil {
		log.Printf("[lunch] ❌ бот: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	chatID, err := getChatID()
	if err != nil {
		log.Printf("[lunch] ❌ chatID: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	quote, author, err := fetchQuote()
	if err != nil {
		log.Printf("[lunch] ❌ цитата: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	text := fmt.Sprintf("🍽 Обеденная цитата:\n\n\u201c%s\u201d\n\n© %s", quote, author)
	if err := sendText(bot, chatID, text); err != nil {
		log.Printf("[lunch] ❌ отправка: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	log.Printf("[lunch] ✅ автор=%q", author)
	fmt.Fprintln(w, "ok")
}

func handleEvening(w http.ResponseWriter, r *http.Request) {
	log.Println("[evening] cron вызван")
	bot, err := getBot()
	if err != nil {
		log.Printf("[evening] ❌ бот: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	chatID, err := getChatID()
	if err != nil {
		log.Printf("[evening] ❌ chatID: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	text := "🌙 День подходит к концу!\n\nНадеемся, он был продуктивным и наполненным 😊\nОтдыхайте, завтра будет новый день — желаем всем спокойного вечера и хорошего отдыха 🌟"
	if err := sendText(bot, chatID, text); err != nil {
		log.Printf("[evening] ❌ отправка: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	log.Println("[evening] ✅ отправлено")
	fmt.Fprintln(w, "ok")
}

func handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusOK)
		return
	}
	bot, err := getBot()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	var update tgbotapi.Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
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
		targetID, err := getChatID()
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ CHAT_ID не задан"))
		} else {
			loc := time.FixedZone("UTC+5", 5*60*60)
			now := time.Now().In(loc)
			day := int(now.Weekday())
			_, week := now.ISOWeek()
			sendText(bot, targetID, morningMessages[day][week%4])
			if q, a, err := fetchQuote(); err == nil {
				sendText(bot, targetID, fmt.Sprintf("🍽 Обеденная цитата:\n\n\u201c%s\u201d\n\n© %s", q, a))
			}
			sendText(bot, targetID, "🌙 День подходит к концу!\n\nНадеемся, он был продуктивным и наполненным 😊\nОтдыхайте, завтра будет новый день — желаем всем спокойного вечера и хорошего отдыха 🌟")
			bot.Send(tgbotapi.NewMessage(chatID, "✅ Все три сообщения отправлены"))
		}
	}

	// /quote
	if msg.IsCommand() && msg.Command() == "quote" {
		q, a, err := fetchQuote()
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Не удалось получить цитату"))
		} else {
			bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("\u201c%s\u201d\n\n© %s", q, a)))
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

// -------------------------------------------------------
// main
// -------------------------------------------------------

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.Println("=== БОТ ЗАПУСКАЕТСЯ ===")

	mux := http.NewServeMux()
	mux.HandleFunc("/api/webhook", handleWebhook)
	mux.HandleFunc("/api/cron-morning", handleMorning)
	mux.HandleFunc("/api/cron-lunch", handleLunch)
	mux.HandleFunc("/api/cron-evening", handleEvening)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Bot is running")
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	log.Printf("Сервер запущен на :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
