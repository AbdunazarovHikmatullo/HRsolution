package bot

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Состояния FSM пользователя
type UserState int

const (
	StateIdle         UserState = iota // ожидание выбора вакансии
	StateWaitingForCV                  // вакансия выбрана, ждём резюме
)

// userSession хранит состояние одного пользователя
type userSession struct {
	State     UserState
	VacancyID string
}

// Handler обрабатывает апдейты Telegram
type Handler struct {
	bot      *tgbotapi.BotAPI
	mu       sync.RWMutex
	sessions map[int64]*userSession
}

// NewHandler создаёт новый Handler
func NewHandler(b *tgbotapi.BotAPI) *Handler {
	return &Handler{
		bot:      b,
		sessions: make(map[int64]*userSession),
	}
}

func (h *Handler) getSession(chatID int64) *userSession {
	h.mu.Lock()
	defer h.mu.Unlock()
	if s, ok := h.sessions[chatID]; ok {
		return s
	}
	s := &userSession{State: StateIdle}
	h.sessions[chatID] = s
	return s
}

// Handle диспетчеризирует входящий апдейт
func (h *Handler) Handle(update tgbotapi.Update) {
	if update.CallbackQuery != nil {
		h.handleCallback(update.CallbackQuery)
		return
	}
	if update.Message == nil {
		return
	}
	if update.Message.IsCommand() {
		h.handleCommand(update.Message)
		return
	}
	if update.Message.Document != nil {
		h.handleDocument(update.Message)
		return
	}
	// Любое другое текстовое сообщение
	h.handleText(update.Message)
}

// ── Команды ──────────────────────────────────────────────────────────────────

func (h *Handler) handleCommand(msg *tgbotapi.Message) {
	switch msg.Command() {
	case "start":
		h.sendWelcome(msg.Chat.ID)
	case "help":
		h.sendHelp(msg.Chat.ID)
	case "vacancies":
		h.sendVacancyMenu(msg.Chat.ID)
	default:
		h.send(msg.Chat.ID, "Неизвестная команда. Используй /start")
	}
}

func (h *Handler) sendWelcome(chatID int64) {
	text := `👋 *Добро пожаловать в HRSolution!*

Я — автоматизированный HR-бот. Я проанализирую ваше резюме и дам честную оценку по выбранной вакансии.

*Как это работает:*
1️⃣ Выбери вакансию
2️⃣ Отправь резюме в формате PDF
3️⃣ Получи оценку и обратную связь от AI

👇 Выбери интересующую тебя вакансию:`

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = buildVacancyKeyboard()
	h.bot.Send(msg)
}

func (h *Handler) sendHelp(chatID int64) {
	text := `ℹ️ *HRSolution — справка*

*Команды:*
/start — главное меню и выбор вакансии
/vacancies — показать список вакансий
/help — эта справка

*Как использовать:*
1. Нажми /start и выбери вакансию
2. Отправь резюме в формате *PDF*
3. Жди — обычно анализ занимает 10-20 секунд

*Важно:* PDF должен содержать текст (не сканированные изображения).`

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	h.bot.Send(msg)
}

func (h *Handler) sendVacancyMenu(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "📋 *Выбери вакансию для оценки резюме:*")
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = buildVacancyKeyboard()
	h.bot.Send(msg)
}

// ── Callback (выбор вакансии) ─────────────────────────────────────────────

func (h *Handler) handleCallback(cb *tgbotapi.CallbackQuery) {
	// Отвечаем на callback чтобы убрать "часики" у кнопки
	h.bot.Request(tgbotapi.NewCallback(cb.ID, ""))

	// Спец. callback-и
	if cb.Data == "_change_vacancy" || cb.Data == "_change_vacancy_result" {
		sess := h.getSession(cb.Message.Chat.ID)
		sess.State = StateIdle
		sess.VacancyID = ""
		h.sendVacancyMenu(cb.Message.Chat.ID)
		return
	}

	// Повторный анализ: "_retry_<vacancyID>"
	vacancyID := cb.Data
	if len(vacancyID) > 7 && vacancyID[:7] == "_retry_" {
		vacancyID = vacancyID[7:]
		sess := h.getSession(cb.Message.Chat.ID)
		vacancy, ok := FindVacancy(vacancyID)
		if !ok {
			h.send(cb.Message.Chat.ID, "Неизвестная вакансия. Попробуй /start")
			return
		}
		sess.State = StateWaitingForCV
		sess.VacancyID = vacancyID
		text := fmt.Sprintf("📎 Отправь новое резюме для вакансии *%s %s*:", vacancy.Emoji, vacancy.Title)
		msg := tgbotapi.NewMessage(cb.Message.Chat.ID, text)
		msg.ParseMode = "Markdown"
		h.bot.Send(msg)
		return
	}

	vacancy, ok := FindVacancy(vacancyID)
	if !ok {
		h.send(cb.Message.Chat.ID, "Неизвестная вакансия. Попробуй /start")
		return
	}

	sess := h.getSession(cb.Message.Chat.ID)
	sess.State = StateWaitingForCV
	sess.VacancyID = vacancyID

	text := fmt.Sprintf(
		"✅ Выбрана вакансия: *%s %s*\n\n📎 Теперь отправь своё резюме в формате *PDF*.\n\n_Убедись, что PDF содержит текст, а не скан изображения._",
		vacancy.Emoji, vacancy.Title,
	)
	msg := tgbotapi.NewMessage(cb.Message.Chat.ID, text)
	msg.ParseMode = "Markdown"
	// Кнопка "Сменить вакансию"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Сменить вакансию", "_change_vacancy"),
		),
	)
	h.bot.Send(msg)
}

// ── Текстовые сообщения ───────────────────────────────────────────────────

func (h *Handler) handleText(msg *tgbotapi.Message) {
	sess := h.getSession(msg.Chat.ID)

	// Обработка кнопки "Сменить вакансию" через callback
	if msg.Text == "_change_vacancy" {
		sess.State = StateIdle
		h.sendVacancyMenu(msg.Chat.ID)
		return
	}

	switch sess.State {
	case StateIdle:
		h.send(msg.Chat.ID, "👋 Привет! Используй /start чтобы выбрать вакансию и начать анализ резюме.")
	case StateWaitingForCV:
		h.send(msg.Chat.ID, "📎 Пожалуйста, отправь резюме в формате *PDF* (кнопка скрепки → Файл).")
	}
}

// ── Документы (PDF) ───────────────────────────────────────────────────────

func (h *Handler) handleDocument(msg *tgbotapi.Message) {
	sess := h.getSession(msg.Chat.ID)

	if sess.State != StateWaitingForCV {
		h.send(msg.Chat.ID, "Сначала выбери вакансию через /start")
		return
	}

	doc := msg.Document
	// Проверяем MIME-тип
	if doc.MimeType != "application/pdf" {
		h.send(msg.Chat.ID, "❗ Принимаются только файлы *PDF*. Пожалуйста, конвертируй резюме и попробуй снова.")
		return
	}

	// Проверяем размер (макс 20 МБ)
	if doc.FileSize > 20*1024*1024 {
		h.send(msg.Chat.ID, "❗ Файл слишком большой. Максимальный размер — 20 МБ.")
		return
	}

	vacancy, _ := FindVacancy(sess.VacancyID)

	// Сообщаем что начали обработку
	processingMsg, _ := h.bot.Send(tgbotapi.NewMessage(msg.Chat.ID,
		fmt.Sprintf("⏳ Анализирую резюме для вакансии *%s %s*...\n\nЭто займёт ~15-30 секунд.", vacancy.Emoji, vacancy.Title),
	))
	_ = processingMsg

	// Скачиваем файл
	filePath, err := h.downloadFile(doc.FileID)
	if err != nil {
		log.Printf("Ошибка скачивания файла: %v", err)
		h.send(msg.Chat.ID, "❗ Не удалось скачать файл. Попробуй ещё раз.")
		return
	}
	defer os.Remove(filePath)

	// Извлекаем текст из PDF
	cvText, err := ExtractTextFromPDF(filePath)
	if err != nil {
		log.Printf("Ошибка извлечения текста из PDF: %v", err)
		h.send(msg.Chat.ID, fmt.Sprintf("❗ Не удалось прочитать PDF:\n_%v_\n\nУбедись, что файл содержит текст, а не сканированные страницы.", err))
		return
	}

	// Отправляем в Gemini
	ctx := context.Background()
	result, err := AnalyzeCV(ctx, vacancy.Title, vacancy.Description, cvText)
	if err != nil {
		log.Printf("Ошибка анализа Gemini: %v", err)
		h.send(msg.Chat.ID, "❗ Ошибка при анализе резюме. Попробуй позже.")
		return
	}

	// Удаляем сообщение "анализирую..."
	h.bot.Request(tgbotapi.NewDeleteMessage(msg.Chat.ID, processingMsg.MessageID))

	// Отправляем результат
	resultMsg := tgbotapi.NewMessage(msg.Chat.ID, result)
	resultMsg.ParseMode = "" // Gemini уже форматирует через эмодзи
	resultMsg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Попробовать другую вакансию", "_change_vacancy_result"),
			tgbotapi.NewInlineKeyboardButtonData("📄 Повторный анализ", "_retry_"+sess.VacancyID),
		),
	)
	h.bot.Send(resultMsg)

	// Сбрасываем состояние (пользователь может сразу отправить новое резюме)
	sess.State = StateIdle
}

// downloadFile скачивает файл из Telegram и сохраняет во временный файл
func (h *Handler) downloadFile(fileID string) (string, error) {
	fileConfig := tgbotapi.FileConfig{FileID: fileID}
	file, err := h.bot.GetFile(fileConfig)
	if err != nil {
		return "", fmt.Errorf("GetFile: %w", err)
	}

	url := file.Link(h.bot.Token)
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("HTTP GET: %w", err)
	}
	defer resp.Body.Close()

	tmpFile, err := os.CreateTemp("", "cv_*.pdf")
	if err != nil {
		return "", fmt.Errorf("создание временного файла: %w", err)
	}
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("запись файла: %w", err)
	}

	return tmpFile.Name(), nil
}

// ── Утилиты ───────────────────────────────────────────────────────────────

func (h *Handler) send(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	if _, err := h.bot.Send(msg); err != nil {
		log.Printf("Ошибка отправки сообщения: %v", err)
	}
}

// buildVacancyKeyboard строит inline-клавиатуру со всеми вакансиями
func buildVacancyKeyboard() tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	// По 2 кнопки в строке
	for i := 0; i < len(Vacancies); i += 2 {
		v1 := Vacancies[i]
		btn1 := tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%s %s", v1.Emoji, v1.Title), v1.ID,
		)

		if i+1 < len(Vacancies) {
			v2 := Vacancies[i+1]
			btn2 := tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("%s %s", v2.Emoji, v2.Title), v2.ID,
			)
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn1, btn2))
		} else {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn1))
		}
	}

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// handleCallback — обработка кнопки "сменить вакансию" из результата
func init() {
	// регистрируем дополнительные callback-и в Handler через monkey patch не нужен —
	// они обрабатываются в handleCallback по префиксу
}