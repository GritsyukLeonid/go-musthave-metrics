package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	models "github.com/GritsyukLeonid/go-musthave-metrics/internal/model"
	"github.com/GritsyukLeonid/go-musthave-metrics/internal/repository"
)

// Update принимает метрику из URL вида /update/<тип>/<имя>/<значение>.
//
// Хендлер держит зависимость интерфейсом, а не *MemStorage: когда
// в девятом инкременте появится хранилище с файлом, менять здесь
// будет нечего.
type Update struct {
	store repository.Repository
}

func NewUpdate(store repository.Repository) *Update {
	return &Update{store: store}
}

// ServeHTTP реализует http.Handler.
//
// Коды ответа заданы инкрементом:
//   - 200 — метрика принята;
//   - 404 — не передано имя метрики;
//   - 400 — неизвестный тип или значение, которое не парсится.
func (h *Update) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// chi.URLParam достаёт именованные сегменты из шаблона маршрута,
	// объявленного в пакете server: роутер кладёт их в контекст запроса,
	// парсить r.URL.Path руками не нужно.
	mType := chi.URLParam(r, "type")
	name := chi.URLParam(r, "name")
	rawValue := chi.URLParam(r, "value")

	// Маршрут без имени («/update/gauge/») не совпадёт с шаблоном и даст 404
	// ещё в роутере. Явная проверка нужна для запросов, которые всё же
	// дошли сюда с пустым сегментом.
	if name == "" {
		http.Error(w, "metric name is required", http.StatusNotFound)
		return
	}

	switch mType {
	case models.Gauge:
		value, err := strconv.ParseFloat(rawValue, 64)
		if err != nil {
			http.Error(w, "gauge value must be a float64: "+rawValue, http.StatusBadRequest)
			return
		}
		h.store.UpdateGauge(name, value)

	case models.Counter:
		delta, err := strconv.ParseInt(rawValue, 10, 64)
		if err != nil {
			http.Error(w, "counter value must be an int64: "+rawValue, http.StatusBadRequest)
			return
		}
		h.store.UpdateCounter(name, delta)

	default:
		http.Error(w, "unknown metric type: "+mType, http.StatusBadRequest)
		return
	}

	// Заголовок выставляем до WriteHeader — после отправки статуса
	// изменения в w.Header() уже никуда не попадут.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
}
