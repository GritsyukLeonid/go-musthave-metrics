package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"

	models "github.com/GritsyukLeonid/go-musthave-metrics/internal/model"
)

const clientTimeout = 5 * time.Second

type Client struct {
	client *resty.Client
}

func NewClient(baseURL string) *Client {
	client := resty.New().
		SetBaseURL(strings.TrimRight(baseURL, "/")).
		SetTimeout(clientTimeout)

	return &Client{client: client}
}

func (c *Client) Send(ctx context.Context, m models.Metrics) error {
	switch m.MType {
	case models.Gauge:
		if m.Value == nil {
			return fmt.Errorf("gauge %s: value is not set", m.ID)
		}
	case models.Counter:
		if m.Delta == nil {
			return fmt.Errorf("counter %s: delta is not set", m.ID)
		}
	default:
		return fmt.Errorf("metric %s: unknown type %q", m.ID, m.MType)
	}

	body, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal metric %s: %w", m.ID, err)
	}

	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	if _, err := zw.Write(body); err != nil {
		return fmt.Errorf("compress metric %s: %w", m.ID, err)
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("compress metric %s: %w", m.ID, err)
	}

	resp, err := c.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("Content-Encoding", "gzip").
		SetHeader("Accept-Encoding", "gzip").
		SetHeader("Accept", "application/json").
		SetBody(compressed.Bytes()).
		Post("/update/")
	if err != nil {
		return fmt.Errorf("send %s: %w", m.ID, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("send %s: unexpected status %s", m.ID, resp.Status())
	}

	return nil
}
