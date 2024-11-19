package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Модель для хранения данных в БД
type CatFood struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    int64     `gorm:"index"`
	Amount    int       `gorm:"check:amount >= 1 AND amount <= 20"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// Глобальная переменная для хранения разрешенных пользователей
var allowedUsers map[int64]bool

func main() {
	// Читаем токен бота и ID разрешенных пользователей
	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatalf("Не указан BOT_TOKEN в переменных окружения")
	}

	userIDs := os.Getenv("ALLOWED_USERS")
	if userIDs == "" {
		log.Fatalf("Не указаны ALLOWED_USERS в переменных окружения")
	}
	allowedUsers = parseAllowedUsers(userIDs)

	// Подключение к базе данных
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatalf("Не указан DATABASE_URL в переменных окружения")
	}

	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("Ошибка подключения к БД: %v", err)
	}

	// Миграции
	if err := db.AutoMigrate(&CatFood{}); err != nil {
		log.Fatalf("Ошибка миграции: %v", err)
	}

	// Создаем Telegram-бота
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalf("Ошибка создания бота: %v", err)
	}

	log.Printf("Бот запущен: %s", bot.Self.UserName)

	// Создаем клавиатуру
	keyboard := createKeyboard()

	// Получаем обновления
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		userID := update.Message.From.ID

		// Проверка на доступ
		if !allowedUsers[userID] {
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "У вас нет доступа к этому боту.")
			bot.Send(msg)
			continue
		}

		// Обработка команд и текста
		text := update.Message.Text
		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "start":
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Привет! Используй /help, чтобы узнать доступные команды.")
				msg.ReplyMarkup = keyboard
				bot.Send(msg)

			case "add":
				args := update.Message.CommandArguments()
				amount, err := strconv.Atoi(strings.TrimSpace(args))
				if err != nil || amount < 1 || amount > 20 {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Пожалуйста, укажи число от 1 до 20. Пример: /add 5")
					bot.Send(msg)
					continue
				}

				err = addCatFood(db, userID, amount)
				if err != nil {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка при добавлении данных.")
					bot.Send(msg)
					log.Printf("Ошибка добавления данных: %v", err)
				} else {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Добавлено %dг корма.", amount))
					bot.Send(msg)
				}

			case "total":
				total, err := getTotalFood(db)
				if err != nil {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка при получении данных.")
					bot.Send(msg)
					log.Printf("Ошибка получения данных: %v", err)
				} else {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Всего добавлено корма: %dг.", total))
					bot.Send(msg)
				}

			case "today":
				totalToday, err := getTotalFoodToday(db)
				if err != nil {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка при получении данных.")
					bot.Send(msg)
					log.Printf("Ошибка получения данных за сегодня: %v", err)
				} else {
					if totalToday == 0 {
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Сегодня записей нет.")
						bot.Send(msg)
					} else {
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("За сегодня добавлено корма: %dг.", totalToday))
						bot.Send(msg)
					}
				}


			case "today_row":
				records, err := getTodayRecords(db, userID)
				if err != nil {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка при получении данных за сегодня.")
					bot.Send(msg)
					log.Printf("Ошибка получения данных за сегодня: %v", err)
				} else {
					if len(records) == 0 {
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Сегодня записей нет.")
						bot.Send(msg)
					} else {
						response := "Записи за сегодня:\n" + strings.Join(records, "\n")
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, response)
						bot.Send(msg)
					}
				}

			case "delete":
				args := update.Message.CommandArguments()
				id, err := strconv.Atoi(strings.TrimSpace(args))
				if err != nil || id <= 0 {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Пожалуйста, укажи корректный ID записи для удаления. Пример: /delete 1")
					bot.Send(msg)
					continue
				}

				err = deleteCatFood(db, uint(id), userID)
				if err != nil {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка при удалении записи. Убедитесь, что ID корректен.")
					bot.Send(msg)
					log.Printf("Ошибка удаления записи: %v", err)
				} else {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Запись с ID %d удалена.", id))
					bot.Send(msg)
				}

			case "help":
				helpText := `
Доступные команды:
- /add <количество>: Добавить корм (число от 1 до 20). Пример: /add 5
- /total: Показать общее количество корма.
- /today: Показать количество корма за сегодня.
- /today_row: Показать все записи за сегодня.
- /delete <id>: Удалить запись по ID. Пример: /delete 1
- /help: Показать это сообщение.
Вы также можете использовать кнопки для взаимодействия.
				`
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, helpText)
				bot.Send(msg)

			default:
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Неизвестная команда. Используй /help для списка команд.")
				bot.Send(msg)
			}
		} else {
			if text == "Посмотреть корм за сегодня" {
				totalToday, err := getTotalFoodToday(db)
				if err != nil {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка при получении данных.")
					bot.Send(msg)
					log.Printf("Ошибка получения данных за сегодня: %v", err)
				} else {
					if totalToday == 0 {
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Сегодня записей нет.")
						bot.Send(msg)
					} else {
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("За сегодня добавлено корма: %dг.", totalToday))
						bot.Send(msg)
					}
				}
			} else {
					amount, err := strconv.Atoi(text)
					if err != nil || amount < 1 || amount > 20 {
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Пожалуйста, выбери число от 1 до 20 или используй кнопку для просмотра общей статистики.")
						msg.ReplyMarkup = keyboard
						bot.Send(msg)
						continue
					}
	
					err = addCatFood(db, userID, amount)
					if err != nil {
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка при добавлении данных.")
						bot.Send(msg)
						log.Printf("Ошибка добавления данных: %v", err)
					} else {
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Добавлено %dг корма.", amount))
						msg.ReplyMarkup = keyboard
						bot.Send(msg)
					}
				}
		}
	}
}

// Создание клавиатуры
func createKeyboard() tgbotapi.ReplyKeyboardMarkup {
	rows := [][]tgbotapi.KeyboardButton{}
	for i := 1; i <= 20; i += 5 {
		row := tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(fmt.Sprintf("%d", i)),
			tgbotapi.NewKeyboardButton(fmt.Sprintf("%d", i+1)),
			tgbotapi.NewKeyboardButton(fmt.Sprintf("%d", i+2)),
			tgbotapi.NewKeyboardButton(fmt.Sprintf("%d", i+3)),
			tgbotapi.NewKeyboardButton(fmt.Sprintf("%d", i+4)),
		)
		rows = append(rows, row)
	}
	// Добавляем кнопку для просмотра корма за сегодня
	rows = append(rows, tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("Посмотреть корм за сегодня"),
	))
	return tgbotapi.NewReplyKeyboard(rows...)
}

// Добавление корма в БД
func addCatFood(db *gorm.DB, userID int64, amount int) error {
	food := CatFood{
		UserID: userID,
		Amount: amount,
	}
	return db.Create(&food).Error
}

// Удаление записи о корме
func deleteCatFood(db *gorm.DB, id uint, userID int64) error {
	return db.Where("id = ? AND user_id = ?", id, userID).Delete(&CatFood{}).Error
}

// Получение общего количества корма
func getTotalFood(db *gorm.DB) (int, error) {
	var total int64
	err := db.Model(&CatFood{}).Select("SUM(amount)").Scan(&total).Error
	return int(total), err
}

// Получение количества корма за сегодня
func getTotalFoodToday(db *gorm.DB) (int, error) {
	// Получаем часовой пояс из окружения
	timeZone := os.Getenv("TIMEZONE")
	if timeZone == "" {
		timeZone = "UTC" // По умолчанию UTC
	}

	loc, err := time.LoadLocation(timeZone)
	if err != nil {
		return 0, fmt.Errorf("ошибка загрузки часового пояса: %v", err)
	}

	// Вычисляем начало текущего дня в заданном часовом поясе
	now := time.Now().In(loc)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	// Подсчёт общего количества
	var total int64
	err = db.Model(&CatFood{}).Where("created_at >= ?", startOfDay).Select("COALESCE(SUM(amount), 0)").Scan(&total).Error
	if err != nil {
	    return 0, fmt.Errorf("ошибка получения данных: %v", err)
	}
	return int(total), nil
}

// Получение всех записей за сегодня
func getTodayRecords(db *gorm.DB, currentUserID int64) ([]string, error) {
	var records []CatFood

	// Получаем часовой пояс из окружения
	timeZone := os.Getenv("TIMEZONE")
	if timeZone == "" {
		timeZone = "UTC" // По умолчанию UTC
	}

	loc, err := time.LoadLocation(timeZone)
	if err != nil {
		return nil, fmt.Errorf("ошибка загрузки часового пояса: %v", err)
	}

	// Вычисляем начало текущего дня в заданном часовом поясе
	now := time.Now().In(loc)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	// Запрос к базе данных
	err = db.Where("created_at >= ?", startOfDay).Order("created_at").Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("ошибка получения данных: %v", err)
	}

	// Форматируем записи с пометкой авторства
	var formattedRecords []string
	for _, record := range records {
		authorMark := ""
		if record.UserID == currentUserID {
			authorMark = " (Вы)"
		}
		formattedRecords = append(formattedRecords, fmt.Sprintf(
			"ID: %d, Корм: %d, Время: %s%s",
			record.ID, record.Amount, record.CreatedAt.In(loc).Format("15:04"), authorMark,
		))
	}

	return formattedRecords, nil
}

// Парсинг разрешенных пользователей из строки окружения
func parseAllowedUsers(userIDs string) map[int64]bool {
	users := make(map[int64]bool)
	for _, idStr := range strings.Split(userIDs, ",") {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err == nil {
			users[id] = true
		}
	}
	return users
}