package main

import (
	"context"
	"encoding/json"
	"fmt"
	"image/color"
	"math"
	"os"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

const (
	defaultPullTime = 60
	nightStart      = 21
	nightEnd        = 6
)

type Metric struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Config struct {
	PrometheusURL string
	PullPeriod    time.Duration
	Metrics       []Metric
}

type myTheme struct {
	Config *Config
}

func loadMetricsFromFile(path string) ([]Metric, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var metrics []Metric
	err = json.Unmarshal(data, &metrics)
	return metrics, err
}

func GetConfig() (*Config, error) {
	pullTime, err := strconv.Atoi(os.Getenv("PULL_DURATION"))
	if err != nil {
		pullTime = defaultPullTime
	}

	metrics, err := loadMetricsFromFile("metrics.json")
	if err != nil {
		return nil, fmt.Errorf("error loading metrics: %v", err)
	}

	return &Config{
		PrometheusURL: os.Getenv("PROMETHEUS_URL"),
		PullPeriod:    time.Duration(pullTime) * time.Second,
		Metrics:       metrics,
	}, nil
}

func (c *Config) getMetricValue(metric string) (int, error) {
	client, err := api.NewClient(api.Config{Address: c.PrometheusURL})
	if err != nil {
		return 0, err
	}

	v1api := v1.NewAPI(client)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, _, err := v1api.Query(ctx, metric, time.Now())
	if err != nil {
		return 0, err
	}

	vectorVal, ok := result.(model.Vector)
	if !ok || len(vectorVal) == 0 {
		return 0, fmt.Errorf("no data for metric: %s", metric)
	}

	value := vectorVal[0].Value * 1
	return int(math.Round(float64(value))), nil
}

func isNight() bool {
	hour := time.Now().Hour()
	return hour >= nightStart || hour < nightEnd
}

func (m *myTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	return theme.DefaultTheme().Color(name, variant)
}
func (m *myTheme) Font(style fyne.TextStyle) fyne.Resource    { return theme.DefaultTheme().Font(style) }
func (m *myTheme) Size(name fyne.ThemeSizeName) float32       { return theme.DefaultTheme().Size(name) }
func (m *myTheme) Icon(name fyne.ThemeIconName) fyne.Resource { return theme.DefaultTheme().Icon(name) }

func main() {
	config, err := GetConfig()
	if err != nil {
		fmt.Printf("Config error: %v\n", err)
		return
	}

	appIcon, _ := fyne.LoadResourceFromURLString("https://raw.githubusercontent.com/eumel8/cluster-app/main/icon.png")
	a := app.New()
	a.SetIcon(appIcon)
	a.Settings().SetTheme(&myTheme{Config: config})

	w := a.NewWindow("Cluster-App")
	w.Resize(fyne.NewSize(800, 600))

	mainLabel := canvas.NewText("Monitoring Cluster Metrics", color.White)
	content := container.NewVBox(mainLabel)
	w.SetContent(content)
	w.Show()

	go func() {
		for {
			var (
				metricLines     []fyne.CanvasObject
				downCount       int
				unavailableData bool
			)

			for _, metric := range config.Metrics {
				val, err := config.getMetricValue(metric.Name)

				var icon *canvas.Text
				var statusText string

				switch {
				case err != nil:
					unavailableData = true
					icon = canvas.NewText("❓", color.Gray{Y: 180})
					statusText = fmt.Sprintf("%s: unknown", metric.Description)

				case val == 1:
					icon = canvas.NewText("✅", color.RGBA{0, 255, 0, 255})
					statusText = fmt.Sprintf("%s: UP", metric.Description)

				case val == 0:
					downCount++
					icon = canvas.NewText("❌", color.RGBA{255, 0, 0, 255})
					statusText = fmt.Sprintf("%s: DOWN", metric.Description)

				default:
					icon = canvas.NewText("❓", color.Gray{Y: 180})
					statusText = fmt.Sprintf("%s: %d", metric.Description, val)
				}

				icon.TextSize = 32
				text := canvas.NewText(statusText, color.White)
				text.TextSize = 24
				text.Alignment = fyne.TextAlignLeading

				metricLines = append(metricLines, container.NewHBox(icon, text))
			}

			var bgColor color.Color
			switch {
			case unavailableData:
				bgColor = color.Black
			case downCount == 0:
				bgColor = color.RGBA{0, 120, 0, 255} // green
			case downCount == 1:
				bgColor = color.RGBA{255, 215, 0, 255} // yellow
			default:
				bgColor = color.RGBA{139, 0, 0, 255} // red
			}

			bg := canvas.NewRectangle(bgColor)
			bg.Resize(w.Canvas().Size())

			timeLabel := canvas.NewText(time.Now().Format("02.01.2006 15:04:05"), color.White)
			timeLabel.Alignment = fyne.TextAlignCenter
			timeLabel.TextSize = 14

			allContent := container.NewVBox(timeLabel)
			allContent.Add(container.NewVBox(metricLines...))

			stack := container.NewMax(bg, allContent)
			w.SetContent(stack)
			fmt.Printf("new pull %s", metricLines)
			time.Sleep(config.PullPeriod)
		}
	}()

	w.Canvas().SetOnTypedKey(func(e *fyne.KeyEvent) {
		if e.Name == fyne.KeyEscape {
			a.Quit()
		}
	})

	a.Run()
}
