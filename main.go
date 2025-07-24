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
	labelTextSize   = 120
	ecoMetricLow    = 40
	ecoMetricHigh   = 80
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

	value := vectorVal[0].Value * 100
	return int(math.Round(float64(value))), nil
}

func isNight() bool {
	hour := time.Now().Hour()
	return hour >= nightStart || hour < nightEnd
}

func (c *Config) colorForMetric(value int) color.Color {
	isNightTime := isNight()

	switch {
	case value <= ecoMetricLow && value > 0:
		if isNightTime {
			return color.RGBA{140, 0, 0, 255} // dark red
		}
		return color.RGBA{255, 0, 0, 255} // red

	case value > ecoMetricLow && value <= ecoMetricHigh:
		if isNightTime {
			return color.RGBA{175, 175, 0, 200} // dark yellow
		}
		return color.RGBA{255, 255, 0, 255} // yellow

	default:
		if isNightTime {
			return color.RGBA{0, 190, 0, 255} // dark green
		}
		return color.RGBA{0, 255, 0, 255} // green
	}
}

func (m *myTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	return theme.DefaultTheme().Color(name, variant)
}
func (m *myTheme) Font(style fyne.TextStyle) fyne.Resource        { return theme.DefaultTheme().Font(style) }
func (m *myTheme) Size(name fyne.ThemeSizeName) float32           { return theme.DefaultTheme().Size(name) }
func (m *myTheme) Icon(name fyne.ThemeIconName) fyne.Resource     { return theme.DefaultTheme().Icon(name) }

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

	mainLabel := canvas.NewText("Monitoring Cluster Metrics", color.White)
	content := container.NewVBox(mainLabel)
	w.SetContent(content)
	w.Show()

	go func() {
		for {
			var metricsOutput []string
			var colorStatus color.Color

			for _, metric := range config.Metrics {
				val, err := config.getMetricValue(metric.Name)
				if err != nil {
					fmt.Printf("Error querying metric %s: %v\n", metric.Description, err)
					continue
				}
				metricsOutput = append(metricsOutput, fmt.Sprintf("%s: %d", metric.Description, val))
				colorStatus = config.colorForMetric(val)
			}

			timeLabel := canvas.NewText(time.Now().Format("02.01.2006 15:04:05"), color.Gray{})
			timeLabel.Alignment = fyne.TextAlignCenter

			metricsText := canvas.NewText(fmt.Sprintf("%s", metricsOutput), colorStatus)
			metricsText.Alignment = fyne.TextAlignCenter
			metricsText.TextStyle.Bold = true
			metricsText.TextSize = labelTextSize

			ui := container.NewVBox(timeLabel, metricsText)
			w.SetContent(ui)
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

