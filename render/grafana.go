package render

import (
	"context"
	"crypto/md5"
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/devopsext/events/common"
	"github.com/grafana-tools/sdk"
	"github.com/prometheus/client_golang/prometheus"
)

var grafanaRenderCount = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "events_grafana_render_count",
	Help: "Count of all grafana renders",
}, []string{""})

type GrafanaOptions struct {
	URL         string
	Timeout     int
	Datasource  string
	ApiKey      string
	Org         string
	Period      int
	ImageWidth  int
	ImageHeight int
}

type GrafanaAliasColors struct {
}

type Grafana struct {
	client  *http.Client
	options GrafanaOptions
}

func (g *Grafana) findDashboard(c *sdk.Client, ctx context.Context, title string) *sdk.Board {
	var tags []string

	boards, err1 := c.SearchDashboards(ctx, title, false, tags...)
	if err1 != nil {
		return nil
	}

	if len(boards) > 0 {
		log.Info(len(boards))

		board, _, err := c.GetDashboardByUID(ctx, boards[0].UID)
		if err != nil {
			log.Info(len(board.Panels))
			return nil
		}
		return &board
	}
	return nil
}

func (g *Grafana) renderImage(imageURL string) ([]byte, error) {

	req, err := http.NewRequest("GET", g.options.URL+imageURL, nil)

	if err != nil {
		return nil, err
	}

	key := fmt.Sprintf("Bearer %s", g.options.ApiKey)

	req.Header.Set("Authorization", key)

	resp, err := g.client.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}

func (g *Grafana) GenerateDashboard(title string, metric string, operator string, value *float64, minutes *int, unit string) ([]byte, string, error) {

	period := g.options.Period
	if minutes != nil {
		period = *minutes
	}

	t := strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
	suffix := fmt.Sprintf("%x", md5.Sum([]byte(t)))

	board := sdk.NewBoard(fmt.Sprintf("%s - %s", title, suffix))
	board.Timezone = "utc"

	to := time.Now().UTC().Unix() * 1000
	from := (to - int64(period*60)*1000)
	board.Time = sdk.Time{From: fmt.Sprintf("now-%dm", period), To: "now"}

	row1 := board.AddRow("")

	graph := sdk.NewGraph(title)
	graph.Datasource = &g.options.Datasource
	graph.Lines = true
	graph.Type = "graph"
	graph.Linewidth = 1
	graph.Fill = 1
	graph.Transparent = false
	graph.AliasColors = GrafanaAliasColors{}

	var yaxes []sdk.Axis
	var min *sdk.FloatString
	var max *sdk.FloatString

	if value != nil {

		vMin := *value
		vMax := *value
		op := "eq"

		if operator != "==" {
			vMin = vMin - (vMin * 5 / 100)
			vMax = vMax + (vMax * 5 / 100)
		}

		switch operator {
		case ">":
			op = "gt"
		case ">=":
			op = "ge"
		case "<=":
			op = "le"
		case "<":
			op = "lg"
		case "==":
			op = "eq"
		case "!=":
			op = "ne"
		}

		var conditons []sdk.AlertCondition
		conditons = append(conditons, sdk.AlertCondition{
			Evaluator: sdk.AlertEvaluator{
				Params: []float64{*value},
				Type:   op,
			},
			Operator: sdk.AlertOperator{
				Type: "and",
			},
			Query: sdk.AlertQuery{
				Params: []string{"A", "5m", "now"},
			},
			Reducer: sdk.AlertReducer{
				Params: []string{},
				Type:   "avg",
			},
			Type: "query",
		})

		graph.Alert = &sdk.Alert{
			Conditions: conditons,
			Frequency:  "1m",
		}

		min = &sdk.FloatString{Value: vMin, Valid: true}
		max = &sdk.FloatString{Value: vMax, Valid: true}
	}

	yaxes = append(yaxes, sdk.Axis{Show: true, Format: unit, LogBase: 1, Decimals: 0, Min: min, Max: max})
	yaxes = append(yaxes, sdk.Axis{Show: false, Format: unit, LogBase: 1, Decimals: 0})

	graph.Yaxes = yaxes

	xaxis := sdk.Axis{Show: true}
	graph.Xaxis = xaxis

	graph.Legend.AlignAsTable = true
	graph.Legend.Avg = true
	graph.Legend.Min = true
	graph.Legend.Max = true
	graph.Legend.RightSide = false
	graph.Legend.Show = true
	graph.Legend.HideEmpty = true
	graph.Legend.HideZero = false
	graph.Legend.Values = true
	graph.Legend.Current = true

	target := sdk.Target{
		RefID:        "A",
		Expr:         metric,
		LegendFormat: metric,
	}
	graph.AddTarget(&target)

	row1.Add(graph)

	c := sdk.NewClient(g.options.URL, g.options.ApiKey, g.client)

	ctx := context.Background()

	/*b := g.findDashboard(c, ctx, "New dashboard Copy")
	if b != nil {
		log.Info(b.ID)
	}*/

	params := sdk.SetDashboardParams{
		FolderID:  -1,
		Overwrite: false,
	}

	status, err := c.SetDashboard(ctx, *board, params)
	if err != nil {
		log.Error(err)
		return nil, "", err
	}

	log.Debug("%s => %s", *status.UID, *status.Slug)

	if len(board.Rows) == 1 && len(board.Rows[0].Panels) == 1 {

		URL := fmt.Sprintf("/render/d-solo/%s/%s?orgId=%s&panelId=%d&from=%d&to=%d&width=%d&height=%d&tz=%s",
			*status.UID, *status.Slug, g.options.Org, board.Rows[0].Panels[0].ID, from, to, g.options.ImageWidth, g.options.ImageHeight, board.Timezone)

		log.Debug("%s", URL)

		bytes, err := g.renderImage(URL)
		if err != nil {
			log.Error(err)
			return nil, "", err
		}

		_, err = c.DeleteDashboard(ctx, *status.Slug)
		if err != nil {
			log.Error(err)
		}
		return bytes, URL, nil
	}
	return nil, "", errors.New("panel is not found")
}

func makeClient(url string, timeout int) *http.Client {

	if common.IsEmpty(url) {

		log.Debug("Grafana url is not defined. Skipped.")
		return nil
	}

	var transport = &http.Transport{
		Dial:                (&net.Dialer{Timeout: time.Duration(timeout) * time.Second}).Dial,
		TLSHandshakeTimeout: time.Duration(timeout) * time.Second,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
	}

	var client = &http.Client{
		Timeout:   time.Duration(timeout) * time.Second,
		Transport: transport,
	}

	return client
}

func NewGrafana(options GrafanaOptions) *Grafana {

	if common.IsEmpty(options.URL) {
		log.Debug("Grafana URL is not defined. Skipped")
		return nil
	}

	return &Grafana{
		client:  common.MakeHttpClient(options.Timeout),
		options: options,
	}
}

func init() {
	prometheus.Register(grafanaRenderCount)
}