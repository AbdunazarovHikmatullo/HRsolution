package main

import (
	"log"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"hrsolution/bot"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Файл .env не найден, читаю переменные окружения напрямую")
	}

	telegramToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if telegramToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN не задан")
	}

	b, err := tgbotapi.NewBotAPI(telegramToken)
	if err != nil {
		log.Fatalf("Ошибка подключения к Telegram: %v", err)
	}

	b.Debug = false
	log.Printf("Бот запущен: @%s", b.Self.UserName)

	handler := bot.NewHandler(b)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.GetUpdatesChan(u)
	for update := range updates {
		go handler.Handle(update)
	}
}