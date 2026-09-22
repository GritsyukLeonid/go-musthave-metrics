package agent

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"

	models "github.com/GritsyukLeonid/go-musthave-metrics/internal/model"
)

// clientTimeout ограничивает одну отправку. Без таймаута клиент ждёт ответ
// вечно, и один зависший сервер остановил бы весь цикл агента.
const clientTimeout = 5 * time.Second

// Client отправляет метрики на сервер в формате инкремента 2:
// POST /update/<тип>/<имя>/<значение> с пустым телом.
type Client struct {
	client *resty.Client
}

// NewClient готовит клиента для адреса вида "http://localhost:8080".
//
// Адрес и общие заголовки задаются один раз на клиенте, а не собираются
// на каждой из двадцати девяти метрик в отчёте.
func NewClient(baseURL string) *Client {
	client := resty.New().
		// Хвостовой слэш в адресе дал бы "//update/..." — путь с пустым
		// сегментом, который сервер отправит в редирект вместо обработки.
		SetBaseURL(strings.TrimRight(baseURL, "/")).
		SetTimeout(clientTimeout).
		SetHeader("Content-Type", "text/plain")

	return &Client{client: client}
}

// Send отправляет одну метрику и возвращает ошибку, если сервер ответил
// не 200 или не ответил вовсе.
func (c *Client) Send(ctx context.Context, m models.Metrics) error {
	value, err := formatValue(m)
	if err != nil {
		return err
	}

	// SetPathParams подставляет сегменты с экранированием (url.PathEscape):
	// имя метрики приходит из карты, а не из констант, и символ "/" в нём
	// поехал бы в путь как разделитель сегментов.
	//
	// Тело не задаётся: в этом формате значение целиком лежит в пути.
	// Resty сам читает и закрывает тело ответа, поэтому соединение
	// возвращается в пул, а не течёт по одному на метрику.
	resp, err := c.client.R().
		SetContext(ctx).
		SetPathParams(map[string]string{
			"type":  m.MType,
			"name":  m.ID,
			"value": value,
		}).
		Post("/update/{type}/{name}/{value}")
	if err != nil {
		return fmt.Errorf("send %s: %w", m.ID, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("send %s: unexpected status %s", m.ID, resp.Status())
	}

	return nil
}

// formatValue переводит значение метрики в сегмент пути.
func formatValue(m models.Metrics) (string, error) {
	switch m.MType {
	case models.Gauge:
		if m.Value == nil {
			return "", fmt.Errorf("gauge %s: value is not set", m.ID)
		}
		// 'f' и точность -1: минимум цифр, по которым ParseFloat вернёт
		// ровно то же число, и без экспоненты, которую неудобно читать в URL.
		return strconv.FormatFloat(*m.Value, 'f', -1, 64), nil

	case models.Counter:
		if m.Delta == nil {
			return "", fmt.Errorf("counter %s: delta is not set", m.ID)
		}
		return strconv.FormatInt(*m.Delta, 10), nil

	default:
		return "", fmt.Errorf("metric %s: unknown type %q", m.ID, m.MType)
	}
}
