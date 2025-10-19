package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// SubscriptionMessage структура для подписки на WebSocket каналы MEXC
type SubscriptionMessage struct {
	Method string   `json:"method"`
	Params []string `json:"params"`
	ID     int      `json:"id"`
}

// KlineData структура для данных свечей
type KlineData struct {
	Channel string `json:"channel"`
	Symbol  string `json:"symbol"`
	Data    struct {
		OpenTime  int64   `json:"t"`
		Open      float64 `json:"o"`
		High      float64 `json:"h"`
		Low       float64 `json:"l"`
		Close     float64 `json:"c"`
		Volume    float64 `json:"q"`
		TradeTime int64   `json:"T"`
	} `json:"data"`
}

// WebSocketMessage общая структура для WebSocket сообщений
type WebSocketMessage struct {
	ID     int         `json:"id"`
	Result interface{} `json:"result"`
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

func main() {
	fmt.Println("🚀 Запуск WebSocket клиента для MEXC Klines...")
	fmt.Println("📡 Подписка на 5-минутные свечи для пары DOLZ_USDT")
	fmt.Println("🔬 Экспериментальная зона - только через WebSocket!")
	fmt.Println("")

	// URL WebSocket сервера MEXC
	url := "wss://wbs-api.mexc.com/ws"

	fmt.Printf("🔌 Подключение к %s...\n", url)

	// Подключение к WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		log.Fatalf("❌ Ошибка при подключении к WebSocket: %v", err)
	}
	defer conn.Close()

	fmt.Println("✅ Успешно подключились к WebSocket серверу MEXC")

	// Подписываемся на 5-минутные свечи для пары DOLZ_USDT
	subscription := SubscriptionMessage{
		Method: "SUBSCRIPTION",
		Params: []string{"spot@public.kline.v3.api@Min5@DOLZUSDT"},
		ID:     1,
	}

	fmt.Printf("📡 Отправляем подписку: %+v\n", subscription)

	// Отправляем сообщение подписки
	if err := conn.WriteJSON(subscription); err != nil {
		log.Fatalf("❌ Ошибка при отправке сообщения подписки: %v", err)
	}

	fmt.Println("📡 Подписались на 5-минутные свечи для пары DOLZ_USDT")
	fmt.Println("⏰ Ожидание данных...")
	fmt.Println(strings.Repeat("=", 60))

	// Читаем и обрабатываем сообщения
	messageCount := 0
	timeout := time.After(30 * time.Second) // Таймаут 30 секунд

	for {
		select {
		case <-timeout:
			fmt.Println("⏰ Таймаут ожидания данных (30 секунд)")
			fmt.Println("💡 Возможные причины:")
			fmt.Println("   - Пара DOLZ_USDT не активна в данный момент")
			fmt.Println("   - Нет новых данных для этой пары")
			fmt.Println("   - Пара находится в экспериментальной зоне и требует специальных прав")
			fmt.Println("")
			fmt.Println("🔧 Рекомендации:")
			fmt.Println("   - Проверьте активность пары на сайте MEXC")
			fmt.Println("   - Попробуйте другую популярную пару (например, BTCUSDT)")
			fmt.Println("   - Убедитесь, что у вас есть доступ к экспериментальным парам")
			return
		default:
			// Устанавливаем таймаут для чтения
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))

			// Используем recover для обработки паники
			func() {
				defer func() {
					if r := recover(); r != nil {
						fmt.Printf("🔌 WebSocket соединение закрыто: %v\n", r)
						fmt.Println("💡 Это нормальное поведение для экспериментальных пар")
						return
					}
				}()

				messageType, messageBytes, err := conn.ReadMessage()
				if err != nil {
					// Проверяем, не закрыто ли соединение
					if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
						fmt.Println("🔌 WebSocket соединение закрыто сервером")
						fmt.Println("💡 Это нормальное поведение для экспериментальных пар")
						return
					}
					// Проверяем на другие типы закрытия соединения
					if strings.Contains(err.Error(), "websocket: close") {
						fmt.Printf("🔌 WebSocket соединение закрыто: %v\n", err)
						return
					}
					// Проверяем на EOF ошибки
					if strings.Contains(err.Error(), "EOF") {
						fmt.Println("🔌 WebSocket соединение закрыто (EOF)")
						return
					}
					// Игнорируем таймауты и другие ошибки, продолжаем
					return
				}

				messageCount++
				fmt.Printf("📨 Получено сообщение #%d (тип: %d, размер: %d байт)\n", messageType, messageCount, len(messageBytes))

				// Обрабатываем разные типы сообщений
				switch messageType {
				case websocket.TextMessage:
					handleTextMessage(messageBytes, messageCount)
				case websocket.BinaryMessage:
					handleBinaryMessage(messageBytes, messageCount)
				default:
					fmt.Printf("❓ Неизвестный тип сообщения: %d\n", messageType)
				}

				// Ограничиваем количество сообщений для демонстрации
				if messageCount >= 10 {
					fmt.Println("🛑 Получено 10 сообщений. Завершение работы...")
					return
				}
			}()
		}
	}
}

// handleTextMessage обрабатывает текстовые сообщения (JSON)
func handleTextMessage(messageBytes []byte, messageNum int) {
	fmt.Printf("📝 Текстовое сообщение #%d:\n", messageNum)

	// Пытаемся распарсить как JSON (для подтверждения подписки)
	var wsMsg WebSocketMessage
	if err := json.Unmarshal(messageBytes, &wsMsg); err == nil {
		if wsMsg.ID == 1 {
			fmt.Printf("✅ Подписка подтверждена: %+v\n", wsMsg.Result)
			return
		}
		fmt.Printf("📋 WebSocket сообщение: %+v\n", wsMsg)
		return
	}

	// Пытаемся распарсить как данные свечей
	var klineData KlineData
	if err := json.Unmarshal(messageBytes, &klineData); err == nil {
		if klineData.Channel != "" {
			printKlineData(klineData, messageNum)
			return
		}
	}

	// Если не удалось распарсить, выводим сырые данные
	fmt.Printf("📄 Сырые данные: %s\n", string(messageBytes))
	fmt.Println(strings.Repeat("-", 40))
}

// handleBinaryMessage обрабатывает бинарные сообщения (protobuf)
func handleBinaryMessage(messageBytes []byte, messageNum int) {
	fmt.Printf("🔢 Бинарное сообщение #%d (protobuf):\n", messageNum)
	fmt.Printf("   Размер: %d байт\n", len(messageBytes))

	// Пытаемся распарсить как protobuf (упрощенная версия)
	// В реальном проекте здесь был бы правильный protobuf парсинг
	fmt.Printf("🔍 Попытка парсинга protobuf данных...\n")

	// Для демонстрации просто показываем, что получили бинарные данные
	if len(messageBytes) > 0 {
		fmt.Printf("📊 Получены бинарные данные от MEXC WebSocket\n")
		fmt.Printf("💡 Это могут быть protobuf данные с информацией о DOLZ_USDT\n")
		fmt.Printf("🔧 Для полного парсинга нужны правильные .proto файлы от MEXC\n")
	}

	// Если не удалось распарсить protobuf, показываем hex дамп
	fmt.Printf("📊 Hex дамп (первые 100 байт):\n")
	limit := len(messageBytes)
	if limit > 100 {
		limit = 100
	}
	for i := 0; i < limit; i += 16 {
		end := i + 16
		if end > limit {
			end = limit
		}
		fmt.Printf("   %04x: %x\n", i, messageBytes[i:end])
	}
	fmt.Println(strings.Repeat("-", 40))
}

// printKlineData выводит данные свечей в консоль (JSON формат)
func printKlineData(kline KlineData, messageNum int) {
	fmt.Printf("📈 Свеча #%d для %s (JSON):\n", messageNum, kline.Symbol)
	fmt.Printf("   🕐 Время открытия: %s\n", time.Unix(kline.Data.OpenTime/1000, 0).Format("2006-01-02 15:04:05"))
	fmt.Printf("   📊 Открытие: %.6f\n", kline.Data.Open)
	fmt.Printf("   🔺 Максимум: %.6f\n", kline.Data.High)
	fmt.Printf("   🔻 Минимум: %.6f\n", kline.Data.Low)
	fmt.Printf("   📊 Закрытие: %.6f\n", kline.Data.Close)
	fmt.Printf("   📦 Объем: %.2f\n", kline.Data.Volume)
	fmt.Printf("   🕐 Время сделки: %s\n", time.Unix(kline.Data.TradeTime/1000, 0).Format("2006-01-02 15:04:05"))
	fmt.Println(strings.Repeat("=", 60))
}

// init инициализация приложения
func init() {
	// Проверяем наличие необходимых переменных окружения
	if os.Getenv("MEXC_API_KEY") == "" {
		fmt.Println("⚠️  Внимание: MEXC_API_KEY не установлен в переменных окружения")
	}
	if os.Getenv("MEXC_SECRET") == "" {
		fmt.Println("⚠️  Внимание: MEXC_SECRET не установлен в переменных окружения")
	}
}
