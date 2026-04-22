package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// -------------------------------------------------------
// ENV
// -------------------------------------------------------

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

// -------------------------------------------------------
// Массив утренних сообщений [7 дней][4 варианта]
// Индекс дня: 0=воскресенье, 1=понедельник, ..., 6=суббота
// Вариант чередуется по номеру недели в году (повтор раз в 4 недели)
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
// Сообщения — каждое в своей функции
// -------------------------------------------------------

// sendMorning — 10:00 — сообщение по дню недели, вариант по номеру недели
func sendMorning(bot *tgbotapi.BotAPI, chatID int64) {
	loc, _ := time.LoadLocation("Asia/Almaty")
	now := time.Now().In(loc)

	day := int(now.Weekday()) // 0–6
	_, week := now.ISOWeek()
	variant := week % 4
	text := morningMessages[day][variant]

	bot.Send(tgbotapi.NewMessage(chatID, text))
	log.Printf("[morning] день=%d вариант=%d отправлено в %d", day, variant, chatID)
}

// sendLunch — 13:00 — цитата дня
func sendLunch(bot *tgbotapi.BotAPI, chatID int64) {
	quote, author, err := fetchQuote()
	if err != nil {
		log.Printf("[lunch] ошибка цитаты: %v", err)
		return
	}
	text := fmt.Sprintf("🍽 Обеденная цитата:\n\n\u201c%s\u201d\n\n© %s", quote, author)
	bot.Send(tgbotapi.NewMessage(chatID, text))
	log.Printf("[lunch] отправлено в %d", chatID)
}

// sendEvening — 20:00 — итог дня
func sendEvening(bot *tgbotapi.BotAPI, chatID int64) {
	text := "🌙 День подходит к концу!\n\nНадеемся, он был продуктивным и наполненным 😊\nОтдыхайте, завтра будет новый день — желаем всем спокойного вечера и хорошего отдыха 🌟"
	bot.Send(tgbotapi.NewMessage(chatID, text))
	log.Printf("[evening] отправлено в %d", chatID)
}

// -------------------------------------------------------
// Планировщик
// -------------------------------------------------------

type scheduledMessage struct {
	hour   int
	minute int
	send   func(bot *tgbotapi.BotAPI, chatID int64)
	label  string
}

func scheduler(bot *tgbotapi.BotAPI) {
	loc, err := time.LoadLocation("Asia/Almaty")
	if err != nil {
		loc = time.FixedZone("UTC+5", 5*60*60)
	}

	chatIDStr := os.Getenv("CHAT_ID")
	if chatIDStr == "" {
		log.Println("CHAT_ID не задан — планировщик отключён")
		return
	}
	var chatID int64
	fmt.Sscanf(chatIDStr, "%d", &chatID)

	// Расписание — меняй время здесь
	schedule := []scheduledMessage{
		{hour: 10, minute: 0, send: sendMorning, label: "morning"},
		{hour: 15, minute: 0, send: sendLunch, label: "lunch"},
		{hour: 20, minute: 0, send: sendEvening, label: "evening"},
	}

	for _, s := range schedule {
		s := s
		go func() {
			for {
				now := time.Now().In(loc)
				next := time.Date(now.Year(), now.Month(), now.Day(), s.hour, s.minute, 0, 0, loc)
				if !now.Before(next) {
					next = next.Add(24 * time.Hour)
				}
				log.Printf("[%s] следующая отправка через %v", s.label, next.Sub(now))
				time.Sleep(time.Until(next))
				s.send(bot, chatID)
			}
		}()
	}

	select {}
}

// -------------------------------------------------------
// Вспомогательные функции
// -------------------------------------------------------

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

	author := result.QuoteAuthor
	if author == "" {
		author = "Неизвестный автор"
	}
	return result.QuoteText, author, nil
}

// -------------------------------------------------------
// main
// -------------------------------------------------------

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

		// Inline-кнопка
		if update.CallbackQuery != nil {
			cb := update.CallbackQuery
			if cb.Data == "dont_click" {
				bot.Request(tgbotapi.NewCallback(cb.ID, "😈 Я же сказал не нажимать!"))
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
		chatID := msg.Chat.ID

		// /test — прогнать все три сообщения сразу
		/*		if msg.IsCommand() && msg.Command() == "test" {
				var targetID int64
				fmt.Sscanf(os.Getenv("CHAT_ID"), "%d", &targetID)
				if targetID == 0 {
					bot.Send(tgbotapi.NewMessage(chatID, "❌ CHAT_ID не задан в .env"))
				} else {
					sendMorning(bot, targetID)
					time.Sleep(500 * time.Millisecond)
					sendLunch(bot, targetID)
					time.Sleep(500 * time.Millisecond)
					sendEvening(bot, targetID)
					bot.Send(tgbotapi.NewMessage(chatID, "✅ Все три сообщения отправлены"))
				}
			}*/

		// /quote — цитата вручную
		/*		if msg.IsCommand() && msg.Command() == "quote" {
				quote, author, err := fetchQuote()
				if err != nil {
					bot.Send(tgbotapi.NewMessage(chatID, "❌ Не удалось получить цитату"))
				} else {
					text := fmt.Sprintf("\u201c%s\u201d\n\n© %s", quote, author)
					bot.Send(tgbotapi.NewMessage(chatID, text))
				}
			}*/

		// /help — кнопка-ловушка
		if msg.IsCommand() && msg.Command() == "help" {
			username := msg.From.FirstName
			if msg.From.UserName != "" {
				username = "@" + msg.From.UserName
			}
			text := fmt.Sprintf("👋 Привет, %s!\n\n⚠️ Внизу кнопка — НЕ НАЖИМАЙ!", username)
			reply := tgbotapi.NewMessage(chatID, text)
			reply.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("🚫 НЕ НАЖИМАЙ СЮДА !", "dont_click"),
				),
			)
			bot.Send(reply)
		}
	}
}
