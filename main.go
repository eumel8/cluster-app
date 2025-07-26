package main

import (
	"bytes"
	"crypto/tls"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image/color"
	"math"
	"net/http"
	"os"
	"os/exec"
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

// Custom transport to add Basic Auth to each request
type basicAuthTransport struct {
	Username  string
	Password  string
	Transport http.RoundTripper
}

// Struct to hold Bitwarden login fields
type BitwardenItem struct {
        Login struct {
                Username string `json:"username"`
                Password string `json:"password"`
        } `json:"login"`
}

// Get BW_SESSION from env
func getSessionToken() string {
        return os.Getenv("BW_SESSION")
}

// Run Bitwarden CLI to get the item JSON
func getBitwardenItemJSON(itemName string) ([]byte, error) {
        cmd := exec.Command("bw", "get", "item", itemName)
        cmd.Env = append(os.Environ(), "BW_SESSION="+getSessionToken())

        var out bytes.Buffer
        cmd.Stdout = &out

        err := cmd.Run()
        if err != nil {
                return nil, err
        }

        return out.Bytes(), nil
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

	prometheus := c.PrometheusURL
        // doing bitwarden stuff here to get prometheus credentials
        itemName := "Prometheus Agent RemoteWrite"
        jsonData, err := getBitwardenItemJSON(itemName)
        if err != nil {
                fmt.Printf("Failed to get item from Bitwarden: %v\n", err)
        }

        var item BitwardenItem
        err = json.Unmarshal(jsonData, &item)
        if err != nil {
                fmt.Printf("Failed to parse Bitwarden JSON: %v\n", err)
        }

        username := item.Login.Username
        password := item.Login.Password

        if os.Getenv("PROMETHEUS_URL") != "" {
                prometheus = os.Getenv("PROMETHEUS_URL")
        }

	customClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		Timeout: 10 * time.Second,
	}

	// Wrap customClient with basic auth
	transportWithAuth := basicAuthTransport{
		Username:  username,
		Password:  password,
		Transport: customClient.Transport,
	}

	// Create Prometheus API client
	client, err := api.NewClient(api.Config{
		Address:      prometheus,
		RoundTripper: &transportWithAuth,
	})

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

	verbose := flag.Bool("v", false, "enable verbose logging")
	flag.Parse()

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
					if *verbose {
						fmt.Printf("[%s] %s: error - %v\n", time.Now().Format("15:04:05"), metric.Description, err)
					}

				case val == 1:
					icon = canvas.NewText("✅", color.RGBA{0, 255, 0, 255})
					statusText = fmt.Sprintf("%s: UP", metric.Description)
					if *verbose {
						fmt.Printf("[%s] %s: UP (value = %d)\n", time.Now().Format("15:04:05"), metric.Description, val)
					}

				case val == 0:
					downCount++
					icon = canvas.NewText("❌", color.RGBA{255, 0, 0, 255})
					statusText = fmt.Sprintf("%s: DOWN", metric.Description)
					if *verbose {
						fmt.Printf("[%s] %s: DOWN (value = %d)\n", time.Now().Format("15:04:05"), metric.Description, val)
					}

				default:
					icon = canvas.NewText("❓", color.Gray{Y: 180})
					statusText = fmt.Sprintf("%s: %d", metric.Description, val)
					if *verbose {
						fmt.Printf("[%s] %s: UNKNOWN (value = %d)\n", time.Now().Format("15:04:05"), metric.Description, val)
					}
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
