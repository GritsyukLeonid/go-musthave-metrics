package handler

import (
	"bytes"
	"html/template"
	"maps"
	"net/http"
	"slices"

	"github.com/GritsyukLeonid/go-musthave-metrics/internal/repository"
)

// listTemplate разбирается один раз при старте процесса, а не на каждый
// запрос. html/template экранирует имена метрик: они приходят из запросов,
// а не из констант, и «<script>» в имени не должен доехать до браузера
// как разметка.
var listTemplate = template.Must(template.New("metrics").Parse(`<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="utf-8">
    <title>Метрики</title>
</head>
<body>
<h1>Метрики</h1>
{{- if and (not .Gauges) (not .Counters)}}
<p>Пока ни одной метрики не собрано.</p>
{{- end}}
{{- with .Gauges}}
<h2>gauge</h2>
<table>
{{- range .}}
    <tr><td>{{.Name}}</td><td>{{.Value}}</td></tr>
{{- end}}
</table>
{{- end}}
{{- with .Counters}}
<h2>counter</h2>
<table>
{{- range .}}
    <tr><td>{{.Name}}</td><td>{{.Value}}</td></tr>
{{- end}}
</table>
{{- end}}
</body>
</html>
`))

// List отдаёт HTML-страницу со всеми известными серверу метриками.
type List struct {
	store repository.Repository
}

func NewList(store repository.Repository) *List {
	return &List{store: store}
}

// metricRow — строка таблицы: значение уже приведено к тексту, чтобы
// форматированием не занимался шаблон.
type metricRow struct {
	Name  string
	Value string
}

// listPage — данные шаблона.
type listPage struct {
	Gauges   []metricRow
	Counters []metricRow
}

// ServeHTTP реализует http.Handler.
func (h *List) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	page := listPage{
		Gauges:   metricRows(h.store.Gauges(), formatGauge),
		Counters: metricRows(h.store.Counters(), formatCounter),
	}

	// Страница сначала собирается в буфер: если шаблон сломается на
	// середине, клиент получит честную 500, а не обрезанный HTML
	// со статусом 200.
	var buf bytes.Buffer
	if err := listTemplate.Execute(&buf, page); err != nil {
		http.Error(w, "cannot render metrics page", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

// metricRows раскладывает карту метрик в строки таблицы.
//
// Порядок фиксирован (по имени): страница не должна перетасовываться
// между обновлениями из-за случайного порядка обхода карты.
func metricRows[V any](values map[string]V, format func(V) string) []metricRow {
	rows := make([]metricRow, 0, len(values))

	for _, name := range slices.Sorted(maps.Keys(values)) {
		rows = append(rows, metricRow{Name: name, Value: format(values[name])})
	}

	return rows
}
