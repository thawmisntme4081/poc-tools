package mcp

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	BASE_URL = "https://trading.vietcap.com.vn/api/chart/OHLCChart/gap-chart"
)

var VCI_HEADERS = map[string]string{
	"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36",
	"Referer":         "https://trading.vietcap.com.vn/",
	"Origin":          "https://trading.vietcap.com.vn/",
	"Accept":          "*/*",
	"Connection":      "keep-alive",
	"Cache-Control":   "no-cache",
	"Accept-Encoding": "gzip, deflate",
	"Content-Type":    "application/json",
}

type VCITimeFrame string

const (
	ONE_DAY    VCITimeFrame = "ONE_DAY"
	ONE_MINUTE VCITimeFrame = "ONE_MINUTE"
	ONE_HOUR   VCITimeFrame = "ONE_HOUR"
)

type vciStockRequest struct {
	TimeFrame VCITimeFrame `json:"timeFrame"`
	Symbols   []string     `json:"symbols"`
	To        int64        `json:"to"`
	CountBack int32        `json:"countBack"`
}

type vciPriceDataResponse struct {
	Symbol string    `json:"symbol"`
	Open   []float64 `json:"o"`
	High   []float64 `json:"h"`
	Low    []float64 `json:"l"`
	Close  []float64 `json:"c"`
	Volume []int64   `json:"v"`
	Time   []string  `json:"t"`
	// AccumulatedVolume and AccumulatedValue are optional fields
	AccumulatedVolume []int64   `json:"accumulatedVolume,omitempty"`
	AccumulatedValue  []float64 `json:"accumulatedValue,omitempty"`
	MinBatchTruncTime string    `json:"minBatchTruncTime"`
}

type StockPriceItem struct {
	Time   time.Time `json:"time"`
	Open   float64   `json:"open"`
	High   float64   `json:"high"`
	Low    float64   `json:"low"`
	Close  float64   `json:"close"`
	Volume int64     `json:"volume"`
}

type StockPrice struct {
	Symbol string           `json:"symbol"`
	Prices []StockPriceItem `json:"prices"`
}

func getStockPrice(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	symbol, err := request.RequireString("symbol")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	timeFrameStr := request.GetString("time_frame", string(ONE_DAY))
	timeFrame := VCITimeFrame(timeFrameStr)
	countBack := request.GetInt("count_back", 10)

	stockRequest := vciStockRequest{
		TimeFrame: timeFrame,
		Symbols:   []string{symbol},
		To:        time.Now().Unix(),
		CountBack: int32(countBack),
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	body, err := json.Marshal(stockRequest)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	http_req, err := http.NewRequestWithContext(ctx, "POST", BASE_URL, bytes.NewBuffer(body))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	// Write headers from VCI_HEADERS
	for k, v := range VCI_HEADERS {
		http_req.Header.Set(k, v)
	}
	// Fetch stock price from VCI
	resp, err := client.Do(http_req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer resp.Body.Close()
	var resp_bytes []byte
	// Decompress if needed
	if resp.Uncompressed {
		resp_bytes, err = io.ReadAll(resp.Body)
		if err != nil {
			return mcp.NewToolResultError("failed to read body: " + err.Error()), nil
		}
	} else {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return mcp.NewToolResultError("failed to create gzip reader: " + err.Error()), nil
		}
		defer gz.Close()
		resp_bytes, err = io.ReadAll(gz)
		if err != nil {
			return mcp.NewToolResultError("failed to read gzipped response body: " + err.Error()), nil
		}
	}
	var priceData []vciPriceDataResponse
	if err := json.Unmarshal(resp_bytes, &priceData); err != nil {
		return mcp.NewToolResultError(fmt.Errorf("failed to unmarshal response: %w, rawresponse: %s", err, string(resp_bytes)).Error()), nil
	}
	if len(priceData) == 0 {
		return mcp.NewToolResultError("no price data found"), nil
	}
	data := priceData[0]
	prices := make([]StockPriceItem, 0, len(data.Time))
	for i := range data.Time {
		unixTime, err := strconv.ParseInt(data.Time[i], 10, 64)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid time format: %v", err)), nil
		}
		if timeFrame != ONE_DAY {
			// Adjust time zone for vietnamese time (UTC+7)
			unixTime += 7 * 3600
		}
		priceItem := StockPriceItem{
			Time:   time.Unix(unixTime, 0),
			Open:   data.Open[i],
			High:   data.High[i],
			Low:    data.Low[i],
			Close:  data.Close[i],
			Volume: data.Volume[i],
		}
		prices = append(prices, priceItem)
	}
	stockPrice := StockPrice{
		Symbol: data.Symbol,
		Prices: prices,
	}
	return mcp.NewToolResultStructuredOnly(stockPrice), nil
}
