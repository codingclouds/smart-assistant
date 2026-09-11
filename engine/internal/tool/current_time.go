package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// CurrentTimeTool 通过公开时间 API 获取准确的实时时间。
// 当模型需要精确到秒的当前时间、时区时间或验证系统时间时使用。
type CurrentTimeTool struct {
	httpClient *http.Client
}

// NewCurrentTimeTool 创建 get_current_time 工具。
func NewCurrentTimeTool() Tool {
	return &CurrentTimeTool{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Definition 返回 get_current_time 工具的定义。
func (t *CurrentTimeTool) Definition() Definition {
	return Definition{
		Name:        "get_current_time",
		Description: "获取当前实时日期和时间。当需要知道准确的当前时间、时区时间，或用户询问\"现在几点\"、\"今天几号\"等问题时使用此工具。返回包含日期、时间、时区和 UNIX 时间戳的结构化数据。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"timezone": map[string]interface{}{
					"type":        "string",
					"description": "可选时区（如 Asia/Shanghai、America/New_York）。不填则使用 IP 自动检测的时区。",
				},
			},
			"required": []string{},
		},
	}
}

// worldTimeResponse 世界时间 API 的响应结构。
type worldTimeResponse struct {
	WeekNumber  int    `json:"week_number"`
	UTCOffset   string `json:"utc_offset"`
	Timezone    string `json:"timezone"`
	DayOfWeek   int    `json:"day_of_week"`
	DayOfYear   int    `json:"day_of_year"`
	DateTime    string `json:"datetime"`
	Abbreviation string `json:"abbreviation"`
	UTCDatetime string `json:"utc_datetime"`
	Unixtime    int64  `json:"unixtime"`
	ClientIP   string `json:"client_ip"`
}

// Execute 获取当前实时时间，优先从 worldtimeapi.org 获取（含时区），失败时退回本地系统时间。
func (t *CurrentTimeTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	apiURL := "https://worldtimeapi.org/api/ip"

	if tz, ok := args["timezone"].(string); ok && tz != "" {
		apiURL = fmt.Sprintf("https://worldtimeapi.org/api/timezone/%s", tz)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "SmartAssistant/1.0")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		// 远程不可达，退回本地系统时间
		log.Printf("[current_time] 远程 API 不可达 (%v)，退回本地系统时间", err)
		return localTimeResult(), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[current_time] 远程 API 返回 HTTP %d，退回本地系统时间", resp.StatusCode)
		return localTimeResult(), nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	var data worldTimeResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("解析时间数据失败: %w", err)
	}

	return formatTimeResult(data.DateTime, data.Timezone, data.UTCOffset, data.DayOfWeek, data.Unixtime), nil
}

// localTimeResult 使用本地系统时间生成返回字符串。
func localTimeResult() string {
	now := time.Now()
	_, offset := now.Zone()
	offsetHours := offset / 3600
	offsetMin := (offset % 3600) / 60
	var offsetStr string
	if offset >= 0 {
		offsetStr = fmt.Sprintf("+%02d:%02d", offsetHours, offsetMin)
	} else {
		offsetStr = fmt.Sprintf("%03d:%02d", offsetHours, offsetMin)
	}

	return formatTimeResult(
		now.Format("2006-01-02T15:04:05"),
		now.Location().String(),
		offsetStr,
		int(now.Weekday()),
		now.Unix(),
	)
}

// formatTimeResult 格式化时间为统一的输出格式。
func formatTimeResult(dateTime, timezone, utcOffset string, dayOfWeek int, unixTime int64) string {
	weekdayNames := []string{"星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六"}
	weekday := weekdayNames[dayOfWeek]

	return fmt.Sprintf(
		"当前时间：%s\n时区：%s（UTC%s）\n星期：%s\nUNIX 时间戳：%d",
		dateTime,
		timezone,
		utcOffset,
		weekday,
		unixTime,
	)
}

var _ Tool = (*CurrentTimeTool)(nil)