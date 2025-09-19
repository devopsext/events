package render

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
	"github.com/devopsext/utils"
	"github.com/grafana-tools/sdk"
)

type GrafanaRenderOptions struct {
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

type GrafanaRender struct {
	client  *http.Client
	options GrafanaRenderOptions
	logger  sreCommon.Logger
	tracer  sreCommon.Tracer
	counter sreCommon.Counter
}

func (g *GrafanaRender) findDashboard(c *sdk.Client, ctx context.Context, title string) *sdk.Board {
	var tags []string

	boards, err1 := c.SearchDashboards(ctx, title, false, tags...)
	if err1 != nil {
		return nil
	}

	if len(boards) > 0 {
		g.logger.Info(len(boards))

		board, _, err := c.GetDashboardByUID(ctx, boards[0].UID)
		if err != nil {
			g.logger.Info(len(board.Panels))
			return nil
		}
		return &board
	}
	return nil
}

func (g *GrafanaRender) apiKeyIsCredentials() bool {

	arr := strings.Split(g.options.ApiKey, ":")
	if len(arr) == 2 {
		return true
	}
	return false
}

func (g *GrafanaRender) renderImage(imageURL string, apiKey string) ([]byte, error) {

	req, err := http.NewRequest("GET", g.options.URL+imageURL, nil)

	if err != nil {
		return nil, err
	}

	method := "Bearer"
	if g.apiKeyIsCredentials() {
		method = "Basic"
		apiKey = base64.StdEncoding.EncodeToString([]byte(apiKey))
	}

	auth := fmt.Sprintf("%s %s", method, apiKey)
	req.Header.Set("Authorization", auth)

	resp, err := g.client.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (g *GrafanaRender) GenerateDashboard(spanCtx sreCommon.TracerSpanContext,
	title string, metric string, operator string, value *float64, minutes *int, unit string) ([]byte, string, error) {

	span := g.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	period := g.options.Period
	if minutes != nil {
		period = *minutes
	}

	t := strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
	suffix := fmt.Sprintf("%x", md5.Sum([]byte(t)))

	boardName := fmt.Sprintf("%s - %s", title, suffix)
	board := sdk.NewBoard(boardName)
	board.Timezone = "utc"

	to := time.Now().UTC().Unix() * 1000
	from := to - int64(period*60)*1000
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

	graph.GraphPanel.Legend.AlignAsTable = true
	graph.GraphPanel.Legend.Avg = true
	graph.GraphPanel.Legend.Min = true
	graph.GraphPanel.Legend.Max = true
	graph.GraphPanel.Legend.RightSide = false
	graph.GraphPanel.Legend.Show = true
	graph.GraphPanel.Legend.HideEmpty = true
	graph.GraphPanel.Legend.HideZero = false
	graph.GraphPanel.Legend.Values = true
	graph.GraphPanel.Legend.Current = true

	target := sdk.Target{
		RefID:        "A",
		Expr:         metric,
		LegendFormat: metric,
	}
	graph.AddTarget(&target)

	row1.Add(graph)

	c, err := sdk.NewClient(g.options.URL, g.options.ApiKey, g.client)
	if err != nil {
		g.logger.SpanError(span, err)
		return nil, "", err
	}
	ctx := context.Background()

	/*b := g.findDashboard(c, ctx, "New dashboard Copy")
	if b != nil {
		g.logger.Info(b.ID)
	}*/

	params := sdk.SetDashboardParams{
		FolderID:  -1,
		Overwrite: false,
	}

	status, err := c.SetDashboard(ctx, *board, params)
	if err != nil {
		g.logger.SpanError(span, err)
		return nil, "", err
	}

	g.logger.SpanDebug(span, "%s => %s", *status.UID, *status.Slug)

	if len(board.Rows) == 1 && len(board.Rows[0].Panels) == 1 {

		URL := fmt.Sprintf("/render/d-solo/%s/%s?orgId=%s&panelId=%d&from=%d&to=%d&width=%d&height=%d&tz=%s",
			*status.UID, *status.Slug, g.options.Org, board.Rows[0].Panels[0].ID, from, to, g.options.ImageWidth, g.options.ImageHeight, board.Timezone)

		g.logger.SpanDebug(span, "%s", URL)

		bytes, err := g.renderImage(URL, g.options.ApiKey)
		if err != nil {
			g.logger.SpanError(span, err)
			return nil, "", err
		}

		g.counter.Inc()

		_, err = c.DeleteDashboard(ctx, *status.Slug)
		if err != nil {
			g.logger.SpanError(span, err)
		}
		return bytes, URL, nil
	}

	err = errors.New("panel is not found")
	return nil, "", err
}

func NewGrafanaRender(options GrafanaRenderOptions, observability *common.Observability) *GrafanaRender {

	logger := observability.Logs()
	if utils.IsEmpty(options.URL) {
		logger.Debug("Grafana render URL is not defined. Skipped")
		return nil
	}

	return &GrafanaRender{
		client:  utils.NewHttpInsecureClient(options.Timeout),
		options: options,
		logger:  logger,
		tracer:  observability.Traces(),
		counter: observability.Metrics().Counter("grafana", "requests", "Count of all grafana outputs", map[string]string{}),
	}
}
