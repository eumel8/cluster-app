package main

import (
	"context"
	"fmt"
	"image/color"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
)

const (
	defaultPullTime = 60
	nightStart      = 21
	nightEnd        = 6
	labelTextSize   = 120 // or 96
	ecoMetricLow    = 40
	ecoMetricHigh   = 80
	modeFullScreen  = true
)

type Config struct {
	prometheusURL string
	metricName1    string
	metricName2    string
	metricName3    string
	metricName4    string
	metricName5    string
	pullPeriod    time.Duration
}

type myTheme struct{}

var _ fyne.Theme = (*myTheme)(nil)

// collect display size to set font size
// "github.com/kbinani/screenshot"
// bounds := screenshot.GetDisplayBounds(0)
// screenWidth := bounds.Dx()
// screenHeight := bounds.Dy()

func GetConfig() (Config, error) {
	pullTime, err := strconv.Atoi(os.Getenv("PULL_DURATION"))
	if err != nil {
		pullTime = defaultPullTime
	}
	return Config{
		prometheusURL: os.Getenv("PROMETHEUS_URL"),
		metricName1:   "up{instance=\"jambo.eumelnet.de\", job=\"blackbox_icmp_v4\"}",
		metricName2:   "up{instance=\"uucp.gnuu.de\", job=\"blackbox_icmp_v4\"}",
		metricName3:   "up{instance=\"www.eumel.de\", job=\"blackbox_https\"}",
		metricName4:   "up{instance=\"ebooks.eumel.de\", job=\"blackbox_https\"}",
		metricName5:   "up{instance=\"blog.eumel.de\", job=\"blackbox_https\"}",
		pullPeriod:    time.Duration(pullTime) * time.Second,
	}, nil
}

func (m myTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (m myTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}

func (m myTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	c, err := GetConfig()
	if err != nil {
		fmt.Printf("Error getting config: %v\n", err)
		return color.Black
	}
	clusterColor1, err := c.ClusterColor(c.metricName1)
	if err != nil {
		fmt.Printf("Error getting cluster color: %v\n", err)
		return color.Black
	}
	clusterColor2, err := c.ClusterColor(c.metricName2)
	if err != nil {
		fmt.Printf("Error getting cluster color: %v\n", err)
		return color.Black
	}
	clusterColor3, err := c.ClusterColor(c.metricName3)
	if err != nil {
		fmt.Printf("Error getting cluster color: %v\n", err)
		return color.Black
	}
	clusterColor4, err := c.ClusterColor(c.metricName4)
	if err != nil {
		fmt.Printf("Error getting cluster color: %v\n", err)
		return color.Black
	}
	clusterColor5, err := c.ClusterColor(c.metricName5)
	if err != nil {
		fmt.Printf("Error getting cluster color: %v\n", err)
		return color.Black
	}
	fmt.Println("ClusterColors: ", clusterColor1, clusterColor2, clusterColor3, clusterColor4, clusterColor5)
	return clusterColor1
}

func (m myTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (c *Config) GetClusterMetric(metric string) (int, error) {
	if c.prometheusURL == "" {
		return 0, fmt.Errorf("PROMETHEUS_URL environment variable is not set")
	}

	client, err := api.NewClient(api.Config{
		Address: c.prometheusURL,
	})
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		return 0, err
	}

	v1api := v1.NewAPI(client)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, _, err := v1api.Query(ctx, metric, time.Now())
	if err != nil {
		fmt.Printf("Error querying Prometheus: %v\n", err)
		return 0, err
	}

	vectorVal, ok := result.(model.Vector)
	if !ok || len(vectorVal) == 0 {
		return 0, fmt.Errorf("no data returned")
	}
	clusterMetric := vectorVal[0].Value * 100
	fmt.Println("clusterMetric: ", clusterMetric)

	// Round the metric to two decimal places
	roundedMetric := math.Round(float64(clusterMetric)*100) / 100

	formatMetric, err := strconv.Atoi(fmt.Sprintf("%.0f", roundedMetric))
	if err != nil {
		fmt.Printf("Error formatting metric: %v\n", err)
		return 0, err
	}
	return formatMetric, nil
}

func (c *Config) ClusterColor(metric string) (color.Color, error) {

	// default color grey
	clusterColor := color.RGBA{125, 125, 125, 255}
	clusterMetric, err := c.GetClusterMetric(metric)

	if err != nil {
		fmt.Printf("Error querying Prometheus: %v\n", err)
		return color.RGBA{}, err
	}

	if clusterMetric <= ecoMetricLow && clusterMetric > 0 {
		if isNight() {
			// dark red/brown
			clusterColor = color.RGBA{140, 0, 0, 255}
		} else {
			// red
			clusterColor = color.RGBA{255, 0, 0, 255}
		}
	} else if clusterMetric > ecoMetricLow && clusterMetric <= ecoMetricHigh {
		if isNight() {
			// dark yellow
			clusterColor = color.RGBA{175, 175, 0, 200}
		} else {
			// light yellow
			clusterColor = color.RGBA{255, 255, 0, 255}
		}
	} else {
		if isNight() {
			// dark green
			clusterColor = color.RGBA{0, 190, 0, 255}
		} else {
			// light green
			clusterColor = color.RGBA{0, 255, 0, 255}
		}
	}
	return clusterColor, nil
}

// find out if it is night to dim the display
func isNight() bool {
	now := time.Now()
	hour := now.Hour()
	return hour >= nightStart || hour < nightEnd
}

func main() {
	c, err := GetConfig()
	if err != nil {
		fmt.Printf("Error reading config: %v\n", err)
		return
	}
	iconResource, err := fyne.LoadResourceFromURLString("https://raw.githubusercontent.com/eumel8/cluster-app/main/icon.png")
	if err != nil {
		fmt.Printf("Failed to load icon", err)
		return
	}

	clusterApp := app.New()
	clusterApp.SetIcon(iconResource)
	clusterWindow := clusterApp.NewWindow("Cluster-App")
	//clusterWindow.SetFullScreen(modeFullScreen)

	mainLabel := canvas.NewText("Show the current cluster emission", color.White)
	mainContent := container.NewVBox(mainLabel)

	go func() {
		for {
			clusterApp.Settings().SetTheme(&myTheme{})
			clusterMetric1, err := c.GetClusterMetric(c.metricName1)
			if err != nil {
				fmt.Printf("Error querying Prometheus: %v\n", err)
			}
			clusterMetric2, err := c.GetClusterMetric(c.metricName2)
			if err != nil {
				fmt.Printf("Error querying Prometheus: %v\n", err)
			}
			clusterMetric3, err := c.GetClusterMetric(c.metricName3)
			if err != nil {
				fmt.Printf("Error querying Prometheus: %v\n", err)
			}
			clusterMetric4, err := c.GetClusterMetric(c.metricName4)
			if err != nil {
				fmt.Printf("Error querying Prometheus: %v\n", err)
			}
			clusterMetric5, err := c.GetClusterMetric(c.metricName5)
			if err != nil {
				fmt.Printf("Error querying Prometheus: %v\n", err)
			}
			currentTime := time.Now().Format("02.01.2006 15:04:05")
			timeLabel := canvas.NewText(currentTime, color.Gray{})
			timeLabel.Alignment = fyne.TextAlignCenter
			clusterLabel := canvas.NewText(fmt.Sprintf("%d %d %d %d %d", clusterMetric1, clusterMetric2, clusterMetric3,clusterMetric4,clusterMetric5), color.Black)
			clusterLabel.TextStyle.Bold = true
			clusterLabel.TextSize = labelTextSize
			clusterLabel.Alignment = fyne.TextAlignCenter
			content := container.NewVBox(timeLabel, clusterLabel)
			clusterLabel.Refresh()
			timeLabel.Refresh()
			clusterWindow.SetContent(content)
			clusterWindow.Canvas().Refresh(content)
			clusterWindow.Canvas().SetOnTypedKey(func(keyEvent *fyne.KeyEvent) {
				if keyEvent.Name == fyne.KeyEscape {
					clusterApp.Quit()
				}
			})
			time.Sleep(c.pullPeriod)
		}
	}()
	clusterWindow.SetContent(mainContent)
	clusterWindow.ShowAndRun()
}
