package agent

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	models "github.com/GritsyukLeonid/go-musthave-metrics/internal/model"
)

// clientTimeout ограничивает одну отправку. Без таймаута http.Client ждёт
// ответ вечно, и один зависший сервер остановил бы весь цикл агента.
const clientTimeout = 5 * time.Second

// Client отправляет метрики на сервер в формате инкремента 2:
// POST /update/<тип>/<имя>/<значение> с пустым телом.
type Client struct {
	baseURL string
	client  *http.Client
}

// NewClient готовит клиента для адреса вида "http://localhost:8080".
func NewClient(baseURL string) *Client {
	return &Client{
		// Хвостовой слэш в адресе дал бы "//update/..." — путь с пустым
		// сегментом, который сервер отправит в редирект вместо обработки.
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: clientTimeout},
	}
}

// Send отправляет одну метрику и возвращает ошибку, если сервер ответил
// не 200 или не ответил вовсе.
func (c *Client) Send(ctx context.Context, m models.Metrics) error {
	value, err := formatValue(m)
	if err != nil {
		return err
	}

	// Имя метрики экранируется: оно приходит из карты, а не из констант,
	// и символ "/" в нём поехал бы в путь как разделитель сегментов.
	endpoint := c.baseURL + "/update/" + m.MType + "/" + url.PathEscape(m.ID) + "/" + value

	// http.NoBody вместо nil: так Content-Length станет 0, а не пропадёт.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, http.NoBody)
	if err != nil {
		return fmt.Errorf("build request for %s: %w", m.ID, err)
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("send %s: %w", m.ID, err)
	}
	// Тело нужно дочитать и закрыть, иначе соединение не вернётся в пул
	// и каждая метрика будет открывать новое TCP-подключение.
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("send %s: unexpected status %s", m.ID, resp.Status)
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
