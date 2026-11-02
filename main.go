package main

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/redis/go-redis/v9"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// -------------------------------------------------------
// Rate Limiter
// -------------------------------------------------------

type rateLimiter struct {
	mu       sync.Mutex
	requests map[int64][]time.Time
}

var limiter = &rateLimiter{
	requests: make(map[int64][]time.Time),
}

func (r *rateLimiter) allow(userID int64, maxRequests int, period time.Duration) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-period)

	fresh := r.requests[userID][:0]
	for _, t := range r.requests[userID] {
		if t.After(cutoff) {
			fresh = append(fresh, t)
		}
	}
	r.requests[userID] = fresh

	if len(fresh) >= maxRequests {
		return false
	}
	r.requests[userID] = append(r.requests[userID], now)
	return true
}

// cronGuard — защита от повторных запросов
type cronGuard struct {
	mu      sync.Mutex
	lastRun map[string]time.Time
}

var guard = &cronGuard{
	lastRun: make(map[string]time.Time),
}

func (g *cronGuard) allow(name string, minInterval time.Duration) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if last, ok := g.lastRun[name]; ok {
		if time.Since(last) < minInterval {
			log.Printf("[%s] ⛔ слишком часто, пропускаю (последний: %v назад)", name, time.Since(last).Round(time.Second))
			return false
		}
	}
	g.lastRun[name] = time.Now()
	return true
}

type Birthday struct {
	Day      int
	Month    time.Month
	Name     string
	Location *time.Location
}

var birthdays = []Birthday{
	{17, time.March, "Майкл", time.FixedZone("UTC+5", 5*60*60)},
	{25, time.April, "Яночка", time.FixedZone("UTC+5", 5*60*60)},
	{15, time.May, "Адиль (Брооок)", time.FixedZone("UTC+5", 5*60*60)},
	{2, time.May, "Руся", time.FixedZone("UTC+5", 5*60*60)},
	{27, time.July, "(Лион)", time.FixedZone("UTC+3", 3*60*60)},
	{2, time.July, "Баха", time.FixedZone("UTC+5", 5*60*60)},
	{2, time.July, "Доник", time.FixedZone("UTC+5", 5*60*60)},
	{3, time.August, "Дина", time.FixedZone("UTC+5", 5*60*60)},
	{8, time.September, "Александр, Санечка, Санёк, Сайлендблок, Салиноид, Саня, Саша, Шурик, Сашок, Сашка, Сошка", time.FixedZone("UTC+5", 5*60*60)},
	{27, time.September, "Надя", time.FixedZone("UTC+4", 3*60*60)},
	{29, time.October, "Даниил", time.FixedZone("UTC+5", 5*60*60)},
	{17, time.October, "Алмас (Мага)", time.FixedZone("UTC+3", 3*60*60)},
	{20, time.January, "Катя", time.FixedZone("UTC+3", 3*60*60)},
}

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
	resp, err := http.Get("https://stoic.tekloon.net/stoic-quote")
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("quote API status: %s", resp.Status)
	}

	var result struct {
		Data struct {
			QuoteText   string `json:"quote"`
			QuoteAuthor string `json:"author"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", err
	}

	if result.Data.QuoteAuthor == "" {
		result.Data.QuoteAuthor = "Неизвестный автор"
	}

	translated, err := translateToRu(result.Data.QuoteText)
	if err != nil {
		log.Printf("[quote] ошибка перевода: %v", err)

		// Если перевод не удался — возвращаем оригинал
		return result.Data.QuoteText, result.Data.QuoteAuthor, nil
	}

	return translated, result.Data.QuoteAuthor, nil
}

/*
	func fetchCatFact() (string, error) {
		resp, err := http.Get("https://catfact.ninja/fact")
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		var result struct {
			Fact string `json:"fact"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return "", err
		}

		translated, err := translateToRu(result.Fact)
		if err != nil {
			return result.Fact, nil
		}
		return translated, nil
	}


*/

func translateToRu(text string) (string, error) {
	reqURL := fmt.Sprintf(
		"https://api.mymemory.translated.net/get?q=%s&langpair=en|ru",
		url.QueryEscape(text),
	)

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("translate status: %s", resp.Status)
	}

	var result struct {
		ResponseData struct {
			TranslatedText string `json:"translatedText"`
		} `json:"responseData"`
		ResponseStatus int `json:"responseStatus"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.ResponseStatus != 200 {
		return "", fmt.Errorf(
			"translate response status: %d",
			result.ResponseStatus,
		)
	}

	if result.ResponseData.TranslatedText == "" {
		return "", fmt.Errorf("пустой перевод")
	}

	return result.ResponseData.TranslatedText, nil
}
func sendBirthdayNotifications(bot *tgbotapi.BotAPI, chatID int64) error {
	nowUTC := time.Now().UTC()

	for _, birthday := range birthdays {
		now := nowUTC.In(birthday.Location)

		if now.Day() != birthday.Day || now.Month() != birthday.Month {
			continue
		}

		sendText(
			bot,
			chatID,
			fmt.Sprintf("С днем рождения %s 🎉", birthday.Name),
		)
	}

	return nil
}

func fetchMeme() (string, error) {
	resp, err := http.Get("https://www.memify.ru/highfive/")
	if err != nil {
		return "", fmt.Errorf("http.Get: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	re := regexp.MustCompile(`src="(https://www\.nvcdn\.memify\.ru/[^"]+\.(?:jpg|jpeg|png|gif))"`)
	matches := re.FindAllStringSubmatch(string(body), -1)

	if len(matches) == 0 {
		return "", fmt.Errorf("картинки не найдены")
	}

	idx := rand.Intn(len(matches))
	imgURL := matches[idx][1]
	log.Printf("[meme] найдено %d картинок, выбрана #%d: %s", len(matches), idx, imgURL)
	return imgURL, nil
}

func downloadImage(imgURL string) ([]byte, error) {
	req, err := http.NewRequest("GET", imgURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.memify.ru/")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("статус: %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func sendMeme(bot *tgbotapi.BotAPI, chatID int64) {
	imgURL, err := fetchMeme()
	if err != nil {
		log.Printf("[meme] ❌ fetchMeme: %v", err)
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Не удалось получить мем"))
		return
	}
	imgData, err := downloadImage(imgURL)
	if err != nil {
		log.Printf("[meme] ❌ download: %v", err)
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Не удалось загрузить картинку"))
		return
	}
	photo := tgbotapi.NewPhoto(chatID, tgbotapi.FileBytes{Name: "meme.jpg", Bytes: imgData})
	photo.Caption = "😂 Мем дня!"
	if _, err := bot.Send(photo); err != nil {
		log.Printf("[meme] ❌ отправка: %v", err)
	} else {
		log.Printf("[meme] ✅ отправлено")
	}
}

// -------------------------------------------------------
// PUBG Mobile News
// -------------------------------------------------------

type pubgNews struct {
	ID    string
	Title string
	Time  string
	Image string
	Link  string
}

func fetchPubgNews() (*pubgNews, error) {
	apiURL := "https://publicfaas.vasdgame.com/hw/backendapi/?namespace=Faas&fn=getPubgmSection&useSign=1&service=pubgmobile&pdr_appid=3157&env=prod&cluster=sg&sign=4004fd36361332ac5eb1f5dda457fb49"

	body := `{
		"userId":"1",
		"sectionType":"3",
		"contentPlat":"h5",
		"type":["3","4","5","6"],
		"lang":["ru"],
		"sortBy":"timeDesc",
		"offset":0,
		"limit":8,
		"sectionId":["91088"],
		"use_default_lang":false
	}`

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	// ---------------- HEADERS ----------------

	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://www.pubgmobile.com")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Priority", "u=1, i")

	req.Header.Set("Referer", "https://www.pubgmobile.com/")

	req.Header.Set(
		"Sec-CH-UA",
		`"Chromium";v="148", "Google Chrome";v="148", "Not/A)Brand";v="99"`,
	)

	req.Header.Set("Sec-CH-UA-Mobile", "?0")
	req.Header.Set("Sec-CH-UA-Platform", `"Windows"`)

	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "cross-site")

	req.Header.Set(
		"User-Agent",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36",
	)

	req.Header.Set("X-Pandora-Env", "PROD")

	// ---------------- CLIENT ----------------

	client := &http.Client{
		Timeout: 20 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// ---------------- BODY ----------------

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	log.Printf("[pubg] status=%s", resp.Status)
	log.Printf("[pubg] body=%s", string(rawBody))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	// ---------------- JSON ----------------

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`

		Data struct {
			Code    int    `json:"code"`
			Message string `json:"message"`

			Data struct {
				Count int `json:"count"`

				List []struct {
					ID            string   `json:"_id"`
					Title         string   `json:"title"`
					CreateTime    string   `json:"createTime"`
					ContentImages []string `json:"contentImages"`
					GroupID       string   `json:"groupId"`
				} `json:"list"`
			} `json:"data"`
		} `json:"data"`
	}

	if err := json.Unmarshal(rawBody, &result); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("api error code=%d", result.Code)
	}

	if len(result.Data.Data.List) == 0 {
		return nil, fmt.Errorf("empty news list")
	}

	item := result.Data.Data.List[0]

	// ---------------- IMAGE ----------------

	image := ""
	if len(item.ContentImages) > 0 {
		image = item.ContentImages[0]
	}

	// ---------------- REDIS ----------------

	rdb := getRedis()
	if rdb != nil {
		key := "pubg:news:" + item.ID

		val, _ := rdb.Get(ctx, key).Result()
		if val == "shown" {
			log.Printf("[pubg] already shown: %s", item.ID)
			return nil, fmt.Errorf("already shown")
		}

		rdb.Set(ctx, key, "shown", 0)
		log.Printf("[pubg] saved news id: %s", item.ID)
	}

	// ---------------- LINK ----------------

	link := fmt.Sprintf(
		"https://www.pubgmobile.com/ru/news_detail/%s.shtml",
		item.GroupID,
	)

	return &pubgNews{
		ID:    item.ID,
		Title: item.Title,
		Time:  item.CreateTime,
		Image: image,
		Link:  link,
	}, nil
}

func sendPubgNews(bot *tgbotapi.BotAPI, chatID int64) {
	news, err := fetchPubgNews()
	if err != nil {
		log.Printf("[pubg] ❌ %v", err)
		return
	}

	text := fmt.Sprintf(
		"🎮 Новость PUBG Mobile\n\n%s\n\n🕐 %s\n\n🔗 [Читать подробнее](%s)",
		news.Title,
		news.Time,
		news.Link,
	)

	// Если есть картинка — отправляем фото
	if news.Image != "" {
		imgData, err := downloadImage(news.Image)
		if err == nil {
			photo := tgbotapi.NewPhoto(chatID, tgbotapi.FileBytes{Name: "pubg.jpg", Bytes: imgData})
			photo.Caption = text
			photo.ParseMode = "Markdown"
			if _, err := bot.Send(photo); err == nil {
				log.Printf("[pubg] ✅ отправлено с фото: %s", news.Title)
				return
			}
		}
	}

	// Fallback — только текст
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	bot.Send(msg)
	log.Printf("[pubg] ✅ отправлено текстом: %s", news.Title)
}

// -------------------------------------------------------
// Единый Cron — один URL, всё расписание внутри
// -------------------------------------------------------

type cronTask struct {
	hour   int
	minute int
	name   string
	run    func(bot *tgbotapi.BotAPI, chatID int64)
}

// Расписание — добавляй задачи сюда
var cronSchedule = []cronTask{
	{
		hour: 10, minute: 0, name: "morning",
		run: func(bot *tgbotapi.BotAPI, chatID int64) {
			loc := time.FixedZone("UTC+5", 5*60*60)
			now := time.Now().In(loc)
			day := int(now.Weekday())
			_, week := now.ISOWeek()
			sendText(bot, chatID, morningMessages[day][week%4])
		},
	},
	{
		hour: 9, minute: 0, name: "pubg",
		run: func(bot *tgbotapi.BotAPI, chatID int64) {
			sendPubgNews(bot, chatID)
		},
	},
	{
		hour: 13, minute: 0, name: "lunch",
		run: func(bot *tgbotapi.BotAPI, chatID int64) {
			q, a, err := fetchQuote()
			if err != nil {
				log.Printf("[cron/lunch] ❌ %v", err)
				return
			}
			sendText(bot, chatID, fmt.Sprintf("🍽 Обеденная цитата:\n\n\u201c%s\u201d\n\n© %s", q, a))
		},
	},
	//{
	//	hour: 14, minute: 0, name: "cat",
	//	run: func(bot *tgbotapi.BotAPI, chatID int64) {
	//		fact, err := fetchCatFact()
	//		if err != nil {
	//			log.Printf("[cron/cat] ❌ %v", err)
	//			return
	//		}
	//		sendText(bot, chatID, "🐱 Факт о котиках:\n\n"+fact)
	//	},
	//},
	{
		hour: 15, minute: 0, name: "weapon",
		run: func(bot *tgbotapi.BotAPI, chatID int64) {

			log.Printf("[cron/weapon] 🚀 START")

			text, imageURL, err := fetchWeaponFact(chatID)
			if err != nil {
				log.Printf("[cron/weapon] ❌ fetchWeaponFact: %v", err)
				return
			}

			log.Printf(
				"[cron/weapon] ✅ fact received, image=%q, textLen=%d",
				imageURL,
				len(text),
			)

			if imageURL != "" {
				log.Printf("[cron/weapon] 📷 отправляю фото")

				photoMsg := tgbotapi.NewPhoto(
					chatID,
					tgbotapi.FileURL(imageURL),
				)

				photoMsg.Caption = text
				photoMsg.ParseMode = "Markdown"

				if _, err := bot.Send(photoMsg); err != nil {
					log.Printf("[cron/weapon] ❌ ошибка отправки фото: %v", err)
				}

				return
			}

			log.Printf("[cron/weapon] 📝 отправляю текст")

			msg := tgbotapi.NewMessage(chatID, text)
			msg.ParseMode = "Markdown"

			if _, err := bot.Send(msg); err != nil {
				log.Printf("[cron/weapon] ❌ ошибка отправки текста: %v", err)
			}
		},
	},
	{
		hour: 0, minute: 0, name: "birthday",
		run: func(bot *tgbotapi.BotAPI, chatID int64) {
			err := sendBirthdayNotifications(bot, chatID)
			if err != nil {
				log.Printf("[cron/birthday] ❌ %v", err)
				return
			}
		},
	},
	/*	{
		hour: 15, minute: 0, name: "space",
		run: func(bot *tgbotapi.BotAPI, chatID int64) {
			text, err := fetchSpaceFact()
			if err != nil {
				log.Printf("[cron/space] ❌ %v", err)
				return
			}
			msg := tgbotapi.NewMessage(chatID, text)
			msg.ParseMode = "Markdown"
			bot.Send(msg)
		},
	},*/
	{
		hour: 19, minute: 0, name: "meme",
		run: func(bot *tgbotapi.BotAPI, chatID int64) {
			sendMeme(bot, chatID)
		},
	},
	{
		hour: 20, minute: 0, name: "evening",
		run: func(bot *tgbotapi.BotAPI, chatID int64) {
			sendText(bot, chatID, "🌙 День подходит к концу!\n\nНадеемся, он был продуктивным и наполненным 😊\nОтдыхайте, завтра будет новый день — желаем всем спокойного вечера и хорошего отдыха 🌟")
		},
	},
}

func handleCron(w http.ResponseWriter, r *http.Request) {
	loc := time.FixedZone("UTC+5", 5*60*60)
	now := time.Now().In(loc)
	h, m := now.Hour(), now.Minute()

	log.Printf("[cron] 🕐 вызван в %02d:%02d Алматы", h, m)

	// Ручной запуск конкретной задачи: /api/cron?job=morning
	if job := r.URL.Query().Get("job"); job != "" {
		for _, task := range cronSchedule {
			if task.name == job {
				bot, err := getBot()
				if err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
				chatID, err := getChatID()
				if err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
				log.Printf("[cron/%s] 🔧 ручной запуск", job)
				task.run(bot, chatID)
				fmt.Fprintln(w, "ok: "+job)
				return
			}
		}
		http.Error(w, "unknown job: "+job, 400)
		return
	}

	// Автоматический режим — ищем задачи по текущему времени
	bot, err := getBot()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	chatID, err := getChatID()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	ran := 0
	for _, task := range cronSchedule {
		if task.hour == h && task.minute == m {
			if !guard.allow(task.name, 30*time.Minute) {
				continue
			}
			log.Printf("[cron/%s] 🔔 совпало %02d:%02d — запускаю", task.name, h, m)
			task.run(bot, chatID)
			ran++
		}
	}

	if ran == 0 {
		log.Printf("[cron] ⏭ %02d:%02d — нет задач", h, m)
	}

	fmt.Fprintln(w, "ok")
}

// -------------------------------------------------------
// Webhook — обработка команд от пользователей
// -------------------------------------------------------

func processMessage(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	userID := msg.From.ID
	log.Printf("[msg] @%s chatID=%d: %q", msg.From.UserName, chatID, msg.Text)

	// Лимит: не более 3 команд в минуту на пользователя
	if msg.IsCommand() && !limiter.allow(userID, 3, time.Minute) {
		log.Printf("[limiter] ⛔ @%s превысил лимит", msg.From.UserName)
		bot.Send(tgbotapi.NewMessage(chatID, "⏳ Слишком много запросов. Подожди минуту."))
		return
	}

	// /test — прогнать все задачи
	if msg.IsCommand() && msg.Command() == "test" {
		targetID, err := getChatID()
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ CHAT_ID не задан"))
			return
		}
		for _, task := range cronSchedule {
			log.Printf("[/test] запускаю %s", task.name)
			task.run(bot, targetID)
		}
		bot.Send(tgbotapi.NewMessage(chatID, "✅ Все задачи выполнены"))
	}

	// /quote
	if msg.IsCommand() && msg.Command() == "quote" {
		q, a, err := fetchQuote()
		if err != nil {

			log.Printf("[цитат] ❌ %v", err)

			bot.Send(tgbotapi.NewMessage(chatID, "❌ Не удалось получить цитату"))
		} else {
			bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("\u201c%s\u201d\n\n© %s", q, a)))
		}
	}

	// /meme
	if msg.IsCommand() && msg.Command() == "meme" {
		sendMeme(bot, chatID)
	}

	// /cat
	/*	if msg.IsCommand() && msg.Command() == "cat" {
		fact, err := fetchCatFact()
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Не удалось получить факт о кошке"))
		} else {
			bot.Send(tgbotapi.NewMessage(chatID, "🐱 Факт о котиках:\n\n"+fact))
		}
	}*/

	// /birthday
	if msg.IsCommand() && msg.Command() == "birthday" {
		err := sendBirthdayNotifications(bot, chatID)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(
				chatID,
				"❌ Не удалось проверить дни рождения: "+err.Error(),
			))
		}
	}

	// /pubg — последняя новость PUBG Mobile
	if msg.IsCommand() && msg.Command() == "pubg" {
		sendPubgNews(bot, chatID)
	}
	//факт об оружии PUBG
	if msg.IsCommand() && msg.Command() == "weapon" {
		text, imageURL, err := fetchWeaponFact(chatID)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Не удалось получить факт об оружии. Попробуйте позже."))
		} else {
			// Отправляем фото с подписью
			if imageURL != "" {
				photoMsg := tgbotapi.NewPhoto(chatID, tgbotapi.FileURL(imageURL))
				photoMsg.Caption = text
				photoMsg.ParseMode = "Markdown"
				bot.Send(photoMsg)
			} else {
				// Если фото нет, отправляем просто текст
				msg := tgbotapi.NewMessage(chatID, text)
				msg.ParseMode = "Markdown"
				bot.Send(msg)
			}
		}
	}

	/*	// /space — факт о космосе
		if msg.IsCommand() && msg.Command() == "space" {
			text, err := fetchSpaceFact()
			if err != nil {
				bot.Send(tgbotapi.NewMessage(chatID, "❌ Не удалось получить факт о космосе"))
			} else {
				msg := tgbotapi.NewMessage(chatID, text)
				msg.ParseMode = "Markdown"
				bot.Send(msg)
			}
		}*/

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

	processMessage(bot, update.Message)
	fmt.Fprintln(w, "ok")
}

// -------------------------------------------------------
// Polling — локальная разработка
// -------------------------------------------------------

func runPolling(bot *tgbotapi.BotAPI) {
	log.Println("[polling] 🔄 режим polling запущен")
	bot.Request(tgbotapi.DeleteWebhookConfig{})

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
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
			continue
		}
		if update.Message == nil {
			continue
		}
		processMessage(bot, update.Message)
	}
}

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
// Redis
// -------------------------------------------------------

var redisClient *redis.Client
var ctx = context.Background()

func getRedis() *redis.Client {
	if redisClient != nil {
		return redisClient
	}
	opt, err := redis.ParseURL(os.Getenv("REDIS_URL"))
	if err != nil {
		log.Printf("[redis] ❌ ParseURL: %v", err)
		return nil
	}
	redisClient = redis.NewClient(opt)
	return redisClient
}

//// isShown — проверяет показывался ли факт уже
//func isShown(key string) bool {
//	rdb := getRedis()
//	if rdb == nil {
//		return false
//	}
//	val, err := rdb.Get(ctx, "space:"+key).Result()
//	return err == nil && val == "shown"
//}

// markShown — помечает факт как показанный
//func markShown(key string) {
//	rdb := getRedis()
//	if rdb == nil {
//		return
//	}
//	// Храним без TTL — никогда не повторяем
//	rdb.Set(ctx, "space:"+key, "shown", 0)
//	log.Printf("[redis] ✅ помечено: space:%s", key)
//}

// resetShown — сбросить все показанные (когда все факты исчерпаны)
//func resetShown() {
//	rdb := getRedis()
//	if rdb == nil {
//		return
//	}
//	keys, err := rdb.Keys(ctx, "space:*").Result()
//	if err != nil {
//		return
//	}
//	if len(keys) > 0 {
//		rdb.Del(ctx, keys...)
//	}
//	log.Printf("[redis] 🔄 сброшено %d записей", len(keys))
//}

// -------------------------------------------------------
// Факты об оружии  PUBG
// -------------------------------------------------------

// ============================================================
// WEAPON STRUCTURES
// ============================================================

// WeaponData - структура оружия из weapons.json.
type WeaponData struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	Ammo          string   `json:"ammo"`
	FireMode      string   `json:"fire_mode"`
	Magazine      int      `json:"magazine"`
	Image         string   `json:"image"`
	Advantages    string   `json:"advantages"`
	Disadvantages string   `json:"disadvantages"`
	Maps          []string `json:"maps"`
	Accessories   []string `json:"accessories"`

	Power     int `json:"power"`
	FireRate  int `json:"fire_rate"`
	Reload    int `json:"reload"`
	Range     int `json:"range"`
	Stability int `json:"stability"`
}

// WeaponFact - готовый факт об оружии.
type WeaponFact struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`

	Weapon   string `json:"weapon"`
	Type     string `json:"type"`
	Stats    string `json:"stats"`
	ImageURL string `json:"image_url"`

	Power     int `json:"power"`
	FireRate  int `json:"fire_rate"`
	Reload    int `json:"reload"`
	Range     int `json:"range"`
	Stability int `json:"stability"`
}

// ============================================================
// GLOBAL DATA
// ============================================================

var (
	weaponFacts []WeaponFact
	weaponsData []WeaponData
)

// ============================================================
// LOAD WEAPONS
// ============================================================

// LoadWeapons загружает weapons.json,
// встроенный в бинарник через go:embed.
func LoadWeapons() error {
	if len(weaponsJSON) == 0 {
		return fmt.Errorf("weapons.json пустой")
	}

	// Очищаем старые данные перед загрузкой.
	weaponsData = nil
	weaponFacts = nil

	if err := json.Unmarshal(weaponsJSON, &weaponsData); err != nil {
		return fmt.Errorf("ошибка парсинга weapons.json: %w", err)
	}

	if len(weaponsData) == 0 {
		return fmt.Errorf("weapons.json не содержит оружия")
	}

	log.Printf(
		"[weapon_facts] 📦 Загружено %d единиц оружия",
		len(weaponsData),
	)

	generateFactsFromWeapons()

	if len(weaponFacts) == 0 {
		return fmt.Errorf("после генерации получилось 0 фактов")
	}

	return nil
}

// ============================================================
// GENERATE FACTS
// ============================================================

// generateFactsFromWeapons генерирует факты
// на основе данных из weapons.json.
func generateFactsFromWeapons() {
	weaponFacts = make([]WeaponFact, 0)

	for _, weapon := range weaponsData {

		// ----------------------------------------------------
		// Тип оружия
		// ----------------------------------------------------

		weaponTypeEmoji := getWeaponTypeEmoji(weapon.Type)

		// ----------------------------------------------------
		// Описание
		// ----------------------------------------------------

		description := ""

		if weapon.Ammo != "" {
			description += fmt.Sprintf(
				"*%s* — это %s, использующая патроны *%s*.",
				weapon.Name,
				weapon.Type,
				weapon.Ammo,
			)
		} else {
			description += fmt.Sprintf(
				"*%s* — это %s.",
				weapon.Name,
				weapon.Type,
			)
		}

		// ----------------------------------------------------
		// Магазин
		// ----------------------------------------------------

		if weapon.Magazine > 0 {
			description += fmt.Sprintf(
				"\n\n• Магазин: *%d* патронов",
				weapon.Magazine,
			)
		}

		// ----------------------------------------------------
		// Режим стрельбы
		// ----------------------------------------------------

		fireModeText := getFireModeText(weapon.FireMode)

		if fireModeText != "" {
			description += fmt.Sprintf(
				"\n• Режим стрельбы: *%s*",
				fireModeText,
			)
		}

		// ----------------------------------------------------
		// Карты
		// ----------------------------------------------------

		if len(weapon.Maps) > 0 {
			description += fmt.Sprintf(
				"\n• Доступна на картах: *%s*",
				strings.Join(weapon.Maps, "*, *"),
			)
		}

		// ----------------------------------------------------
		// Преимущества
		// ----------------------------------------------------

		if weapon.Advantages != "" {
			description += fmt.Sprintf(
				"\n\n✅ *Преимущества:*\n%s",
				weapon.Advantages,
			)
		}

		// ----------------------------------------------------
		// Недостатки
		// ----------------------------------------------------

		if weapon.Disadvantages != "" {
			description += fmt.Sprintf(
				"\n\n⚠️ *Недостатки:*\n%s",
				weapon.Disadvantages,
			)
		}

		// ----------------------------------------------------
		// Основной факт
		// ----------------------------------------------------

		fact := WeaponFact{
			ID: fmt.Sprintf(
				"%s_fact1",
				weapon.ID,
			),

			Title: fmt.Sprintf(
				"%s %s",
				weaponTypeEmoji,
				weapon.Name,
			),

			Content: description,

			Weapon:   weapon.Name,
			Type:     weapon.Type,
			ImageURL: weapon.Image,

			Power:     weapon.Power,
			FireRate:  weapon.FireRate,
			Reload:    weapon.Reload,
			Range:     weapon.Range,
			Stability: weapon.Stability,
		}

		weaponFacts = append(
			weaponFacts,
			fact,
		)

		// ----------------------------------------------------
		// Факт про обвесы
		// ----------------------------------------------------

		if len(weapon.Accessories) > 0 {

			accessoriesList := strings.Join(
				weapon.Accessories,
				"\n• ",
			)

			content2 := fmt.Sprintf(
				"*%s* поддерживает установку следующего снаряжения:\n\n• %s",
				weapon.Name,
				accessoriesList,
			)

			fact2 := WeaponFact{
				ID: fmt.Sprintf(
					"%s_fact2",
					weapon.ID,
				),

				Title: fmt.Sprintf(
					"🔧 Снаряжение для %s",
					weapon.Name,
				),

				Content: content2,

				Weapon:   weapon.Name,
				Type:     weapon.Type,
				ImageURL: weapon.Image,

				Power:     weapon.Power,
				FireRate:  weapon.FireRate,
				Reload:    weapon.Reload,
				Range:     weapon.Range,
				Stability: weapon.Stability,
			}

			weaponFacts = append(
				weaponFacts,
				fact2,
			)
		}
	}

	log.Printf(
		"[weapon_facts] ✅ Сгенерировано %d фактов об оружии",
		len(weaponFacts),
	)
}

// ============================================================
// WEAPON TYPE EMOJI
// ============================================================

func getWeaponTypeEmoji(weaponType string) string {
	switch {
	case strings.Contains(weaponType, "Штурмовая"):
		return "🔫"

	case strings.Contains(weaponType, "Снайперская"):
		return "🎯"

	case strings.Contains(weaponType, "Пистолет-пулемет"):
		return "💨"

	case strings.Contains(weaponType, "Дробовик"):
		return "💥"

	case strings.Contains(weaponType, "Пулемет"):
		return "🔥"

	default:
		return "🔫"
	}
}

// ============================================================
// FIRE MODE
// ============================================================

func getFireModeText(mode string) string {
	switch mode {

	case "ОДИНОЧНЫЙ/АВТОМАТИЧЕСКИЙ":
		return "одиночный и автоматический"

	case "ОДИНОЧНЫЙ/ОЧЕРЕДЬ/АВТОМАТИЧЕСКИЙ":
		return "одиночный, очередь и автоматический"

	case "ОДИНОЧНЫЙ":
		return "только одиночный"

	case "АВТОМАТИЧЕСКИЙ":
		return "автоматический"

	default:
		return strings.ToLower(mode)
	}
}

// ============================================================
// RANDOM WEAPON FACT
// ============================================================

func getRandomWeaponFact(chatID int64) (WeaponFact, error) {

	if len(weaponFacts) == 0 {
		return WeaponFact{}, fmt.Errorf(
			"нет доступных фактов об оружии",
		)
	}

	redisClient := getRedis()

	// --------------------------------------------------------
	// Redis недоступен
	// --------------------------------------------------------

	if redisClient == nil {
		return weaponFacts[rand.Intn(len(weaponFacts))], nil
	}

	usedKey := fmt.Sprintf(
		"weapon_facts:used:%d",
		chatID,
	)

	// --------------------------------------------------------
	// Получаем использованные факты
	// --------------------------------------------------------

	usedIDs, err := redisClient.
		SMembers(ctx, usedKey).
		Result()

	if err != nil {
		log.Printf(
			"[weapon_facts] ❌ Ошибка получения использованных фактов: %v",
			err,
		)

		return weaponFacts[rand.Intn(len(weaponFacts))], nil
	}

	usedMap := make(map[string]bool)

	for _, id := range usedIDs {
		usedMap[id] = true
	}

	// --------------------------------------------------------
	// Все факты уже использованы
	// --------------------------------------------------------

	if len(usedMap) >= len(weaponFacts) {

		log.Printf(
			"[weapon_facts] 🔄 Все факты использованы, сбрасываем список для чата %d",
			chatID,
		)

		if err := redisClient.Del(ctx, usedKey).Err(); err != nil {
			log.Printf(
				"[weapon_facts] ⚠️ Ошибка сброса: %v",
				err,
			)
		}

		usedMap = make(map[string]bool)
	}

	// --------------------------------------------------------
	// Выбираем доступные факты
	// --------------------------------------------------------

	availableFacts := make(
		[]WeaponFact,
		0,
		len(weaponFacts),
	)

	for _, fact := range weaponFacts {

		if !usedMap[fact.ID] {
			availableFacts = append(
				availableFacts,
				fact,
			)
		}
	}

	// --------------------------------------------------------
	// Дополнительная защита
	// --------------------------------------------------------

	if len(availableFacts) == 0 {

		if err := redisClient.Del(ctx, usedKey).Err(); err != nil {

			log.Printf(
				"[weapon_facts] ⚠️ Ошибка сброса: %v",
				err,
			)

			return weaponFacts[rand.Intn(len(weaponFacts))], nil
		}

		availableFacts = weaponFacts
	}

	// --------------------------------------------------------
	// Выбираем случайный факт
	// --------------------------------------------------------

	selectedFact := availableFacts[rand.Intn(len(availableFacts))]

	// --------------------------------------------------------
	// Сохраняем использованный факт
	// --------------------------------------------------------

	if err := redisClient.
		SAdd(ctx, usedKey, selectedFact.ID).
		Err(); err != nil {

		log.Printf(
			"[weapon_facts] ⚠️ Ошибка сохранения факта: %v",
			err,
		)
	}

	// --------------------------------------------------------
	// TTL
	// --------------------------------------------------------

	if err := redisClient.
		Expire(ctx, usedKey, 24*time.Hour).
		Err(); err != nil {

		log.Printf(
			"[weapon_facts] ⚠️ Ошибка установки TTL: %v",
			err,
		)
	}

	return selectedFact, nil
}

// ============================================================
// FORMAT WEAPON FACT
// ============================================================

func formatWeaponFact(fact WeaponFact) string {

	var builder strings.Builder

	// --------------------------------------------------------
	// Заголовок
	// --------------------------------------------------------

	builder.WriteString(
		fmt.Sprintf(
			"*%s*\n",
			fact.Title,
		),
	)

	builder.WriteString(
		"────────────────────\n",
	)

	// --------------------------------------------------------
	// Описание
	// --------------------------------------------------------

	builder.WriteString(
		fact.Content,
	)

	builder.WriteString("\n\n")

	// --------------------------------------------------------
	// Характеристики
	// --------------------------------------------------------

	builder.WriteString(
		"📊 *Характеристики:*\n\n",
	)

	builder.WriteString("```\n")

	builder.WriteString(
		fmt.Sprintf(
			"💥 Мощность:          %d/100\n",
			fact.Power,
		),
	)

	builder.WriteString(
		fmt.Sprintf(
			"⚡ Скорострельность:  %d/100\n",
			fact.FireRate,
		),
	)

	builder.WriteString(
		fmt.Sprintf(
			"🎯 Стабильность:      %d/100\n",
			fact.Stability,
		),
	)

	builder.WriteString(
		fmt.Sprintf(
			"📏 Дальность:         %d/100\n",
			fact.Range,
		),
	)

	builder.WriteString(
		fmt.Sprintf(
			"🔄 Перезарядка:       %d/100\n",
			fact.Reload,
		),
	)

	builder.WriteString("```\n\n")

	return builder.String()
}

// ============================================================
// FETCH WEAPON FACT
// ============================================================

func fetchWeaponFact(chatID int64) (string, string, error) {

	fact, err := getRandomWeaponFact(chatID)

	if err != nil {
		return "", "", err
	}

	return formatWeaponFact(fact), fact.ImageURL, nil
}

// ============================================================
// RESET WEAPON FACTS
// ============================================================

func resetWeaponFacts(chatID int64) error {

	redisClient := getRedis()

	if redisClient == nil {
		return fmt.Errorf(
			"Redis не доступен",
		)
	}

	usedKey := fmt.Sprintf(
		"weapon_facts:used:%d",
		chatID,
	)

	return redisClient.
		Del(ctx, usedKey).
		Err()
}

// ============================================================
// WEAPON FACTS STATS
// ============================================================

func getWeaponFactsStats(chatID int64) (int, int, error) {

	redisClient := getRedis()

	if redisClient == nil {
		return 0, 0, fmt.Errorf(
			"Redis не доступен",
		)
	}

	usedKey := fmt.Sprintf(
		"weapon_facts:used:%d",
		chatID,
	)

	usedCount, err := redisClient.
		SCard(ctx, usedKey).
		Result()

	if err != nil {
		return 0, 0, err
	}

	return int(usedCount), len(weaponFacts), nil
}

// -------------------------------------------------------
// main
// -------------------------------------------------------

//go:embed weapons.json
var weaponsJSON []byte

// ============================================================
// MAIN
// ============================================================

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	log.Println("=== БОТ ЗАПУСКАЕТСЯ ===")

	loadEnv(".env")

	// --------------------------------------------------------
	// Загружаем weapons.json
	// --------------------------------------------------------

	if err := LoadWeapons(); err != nil {
		log.Printf("[main] ❌ Факты об оружии НЕ загружены: %v", err)
	} else {
		log.Printf(
			"[main] ✅ Факты об оружии загружены: %d фактов",
			len(weaponFacts),
		)
	}

	// --------------------------------------------------------
	// Создаём Telegram Bot
	// --------------------------------------------------------

	bot, err := getBot()
	if err != nil {
		log.Fatalf("[main] ❌ бот: %v", err)
	}

	log.Printf("[main] ✅ бот: @%s", bot.Self.UserName)

	// --------------------------------------------------------
	// Local
	// --------------------------------------------------------

	if os.Getenv("LOCAL") == "true" {
		runPolling(bot)
		return
	}

	// --------------------------------------------------------
	// Vercel HTTP
	// --------------------------------------------------------

	mux := http.NewServeMux()

	mux.HandleFunc("/api/webhook", handleWebhook)
	mux.HandleFunc("/api/cron", handleCron)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Bot is running")
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("[main] 🌐 сервер на :%s", port)

	log.Fatal(http.ListenAndServe(":"+port, mux))
}
