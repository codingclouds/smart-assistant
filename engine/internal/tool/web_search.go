package tool

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// 常量
// =============================================================================

const (
	defaultSearchCount = 5
	maxSearchCount     = 10
	defaultTimeoutSec  = 30 // IQS 较慢
	defaultCacheTTLMin = 5
	maxResultBodyBytes = 256 * 1024
	maxErrorBodyBytes  = 64 * 1024

	// AliCloud IQS API
	iqsHost    = "iqs.cn-zhangjiakou.aliyuncs.com"
	iqsVersion = "2024-11-11"

	// clawbot 配置路径
	clawbotConfigPath = ".openclaw/openclaw.json"
)

// =============================================================================
// 缓存
// =============================================================================

type cacheEntry struct {
	value     map[string]interface{}
	expiresAt time.Time
}

var (
	searchCache   = make(map[string]cacheEntry)
	searchCacheMu sync.RWMutex
)

func readCache(key string) (map[string]interface{}, bool) {
	searchCacheMu.RLock()
	defer searchCacheMu.RUnlock()
	entry, ok := searchCache[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.value, true
}

func writeCache(key string, value map[string]interface{}, ttl time.Duration) {
	searchCacheMu.Lock()
	defer searchCacheMu.Unlock()
	searchCache[key] = cacheEntry{value: value, expiresAt: time.Now().Add(ttl)}
}

// =============================================================================
// WebSearchTool — 使用阿里云 IQS UnifiedSearch（复用 clawbot 实现）
// =============================================================================

// WebSearchTool 通过阿里云 IQS 统一搜索实现网页搜索。
// 直接复用 clawbot haier_web_search 的实现：AliCloud IQS SDK → UnifiedSearch API。
type WebSearchTool struct {
	httpClient    *http.Client
	accessKeyID   string
	accessKeySecret string
}

// NewWebSearchTool 创建 web_search 工具，自动从 clawbot 配置读取 IQS 凭证。
func NewWebSearchTool() Tool {
	t := &WebSearchTool{
		httpClient: &http.Client{
			Timeout: defaultTimeoutSec * time.Second,
		},
	}
	t.loadClawbotCredentials()
	return t
}

// loadClawbotCredentials 从 ~/.openclaw/openclaw.json 读取 Haier IQS 凭证。
func (t *WebSearchTool) loadClawbotCredentials() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("[web_search] 无法获取 home 目录: %v", err)
		return
	}
	configFile := filepath.Join(home, clawbotConfigPath)
	data, err := os.ReadFile(configFile)
	if err != nil {
		log.Printf("[web_search] 无法读取 clawbot 配置 %s: %v", configFile, err)
		return
	}

	var cfg struct {
		Plugins struct {
			Entries map[string]struct {
				Config struct {
					WebSearch struct {
						APIKey string `json:"apiKey"`
					} `json:"webSearch"`
				} `json:"config"`
			} `json:"entries"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("[web_search] 解析 clawbot 配置失败: %v", err)
		return
	}

	entry, ok := cfg.Plugins.Entries["haier_web_search"]
	if !ok {
		log.Printf("[web_search] clawbot 配置中未找到 haier_web_search 插件")
		return
	}

	apiKey := entry.Config.WebSearch.APIKey
	if apiKey == "" {
		log.Printf("[web_search] haier_web_search apiKey 为空")
		return
	}

	parts := strings.SplitN(apiKey, "|", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		log.Printf("[web_search] haier_web_search apiKey 格式无效（期望 AccessKeyId|AccessKeySecret）")
		return
	}

	t.accessKeyID = parts[0]
	t.accessKeySecret = parts[1]
	log.Printf("[web_search] Haier IQS 凭证已从 clawbot 配置加载")
}

// Definition 返回 web_search 工具定义 — 与 clawbot 的 schema 对齐。
func (t *WebSearchTool) Definition() Definition {
	return Definition{
		Name: "web_search",
		Description: "Search the web for real-time information. " +
			"Use this tool when you need to find current news, facts, data, or any information " +
			"that may have changed since your training cutoff. " +
			"Returns titles, URLs, snippets, and relevance scores from search results.",
		Parameters: map[string]interface{}{
			"type": "object",
			"required": []string{"query"},
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query string (max 1024 characters).",
				},
				"count": map[string]interface{}{
					"type":        "integer",
					"description": "Number of results to return (1-10, default 5).",
					"minimum":     1,
					"maximum":     maxSearchCount,
				},
				"time_range": map[string]interface{}{
					"type":        "string",
					"description": "Time filter: OneDay, OneWeek, OneMonth, OneYear, or NoLimit.",
					"enum":        []string{"OneDay", "OneWeek", "OneMonth", "OneYear", "NoLimit"},
				},
				"country": map[string]interface{}{
					"type":        "string",
					"description": "2-letter country code (e.g. 'us', 'cn').",
				},
				"language": map[string]interface{}{
					"type":        "string",
					"description": "ISO 639-1 language code (e.g. 'en', 'zh').",
				},
				"freshness": map[string]interface{}{
					"type":        "string",
					"description": "Time filter: 'day', 'week', 'month', or 'year'.",
				},
				"date_after": map[string]interface{}{
					"type":        "string",
					"description": "Only return results published after this date (YYYY-MM-DD).",
				},
				"date_before": map[string]interface{}{
					"type":        "string",
					"description": "Only return results published before this date (YYYY-MM-DD).",
				},
			},
		},
	}
}

// =============================================================================
// searchResult — 搜索结果
// =============================================================================

type searchResult struct {
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	Snippet     string  `json:"snippet"`
	Description string  `json:"description,omitempty"`
	Score       float64 `json:"score,omitempty"`
	SiteName    string  `json:"siteName,omitempty"`
}

// =============================================================================
// Execute — IQS UnifiedSearch（复用 clawbot haier_web_search 实现）
// =============================================================================

func (t *WebSearchTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	params, err := parseSearchArgs(args)
	if err != nil {
		return "", err
	}

	log.Printf("[web_search] query=%q count=%d time_range=%q",
		params.query, params.count, params.timeRange)

	// 凭证校验
	if t.accessKeyID == "" || t.accessKeySecret == "" {
		return "", fmt.Errorf(
			"Haier IQS 凭证未配置。请在 clawbot 配置中设置 " +
				"plugins.entries.haier_web_search.config.webSearch.apiKey，格式为 " +
				"\"AccessKeyId|AccessKeySecret\"",
		)
	}

	// 缓存查询
	cacheKey := buildCacheKey(params)
	if cached, ok := readCache(cacheKey); ok {
		log.Printf("[web_search] cache hit: key=%s", cacheKey)
		return formatSearchOutput(cached), nil
	}

	// 执行 IQS 搜索
	startedAt := time.Now()
	results, err := t.searchIQS(ctx, params)
	if err != nil {
		return "", fmt.Errorf("IQS 搜索失败: %w", err)
	}

	payload := map[string]interface{}{
		"query":    params.query,
		"provider": "haier_web_search",
		"count":    len(results),
		"tookMs":   time.Since(startedAt).Milliseconds(),
		"results":  results,
	}

	writeCache(cacheKey, payload, time.Duration(params.cacheTTLMinutes)*time.Minute)

	return formatSearchOutput(payload), nil
}

// =============================================================================
// 参数类型与解析
// =============================================================================

type searchParams struct {
	query           string
	count           int
	timeRange       string
	freshness       string
	dateAfter       string
	dateBefore      string
	country         string
	language        string
	timeoutSeconds  int
	cacheTTLMinutes int
}

func parseSearchArgs(args map[string]interface{}) (*searchParams, error) {
	p := &searchParams{
		count:           defaultSearchCount,
		timeRange:       "NoLimit",
		timeoutSeconds:  defaultTimeoutSec,
		cacheTTLMinutes: defaultCacheTTLMin,
	}

	// query (required)
	q, ok := args["query"].(string)
	if !ok || strings.TrimSpace(q) == "" {
		return nil, fmt.Errorf("query is required and must be a non-empty string")
	}
	p.query = strings.TrimSpace(q)

	// count (optional, 1-10)
	p.count = readPositiveInt(args, "count", defaultSearchCount, 1, maxSearchCount)

	// time_range → IQS 原生支持
	if tr, ok := args["time_range"].(string); ok && tr != "" {
		p.timeRange = strings.TrimSpace(tr)
	}

	// freshness → 映射到 time_range（兼容 OpenAI 定义的参数名）
	if f, ok := args["freshness"].(string); ok && f != "" {
		p.freshness = strings.TrimSpace(f)
		freshnessToTimeRange := map[string]string{
			"day":   "OneDay",
			"week":  "OneWeek",
			"month": "OneMonth",
			"year":  "OneYear",
		}
		if tr, ok := freshnessToTimeRange[p.freshness]; ok {
			p.timeRange = tr
		}
	}

	// date_after / date_before（IQS 不直接支持，保留参数）
	if da, ok := args["date_after"].(string); ok {
		p.dateAfter = strings.TrimSpace(da)
	}
	if db, ok := args["date_before"].(string); ok {
		p.dateBefore = strings.TrimSpace(db)
	}

	// country / language（IQS 不直接支持，保留参数）
	if c, ok := args["country"].(string); ok {
		p.country = strings.TrimSpace(c)
	}
	if l, ok := args["language"].(string); ok {
		p.language = strings.TrimSpace(l)
	}

	return p, nil
}

func readPositiveInt(args map[string]interface{}, key string, fallback, min, max int) int {
	val, ok := args[key]
	if !ok {
		return fallback
	}
	var n int
	switch v := val.(type) {
	case float64:
		n = int(v)
	case int:
		n = v
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return fallback
		}
		n = int(i)
	default:
		return fallback
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

func buildCacheKey(p *searchParams) string {
	return fmt.Sprintf("iqs:%s:%d:%s", p.query, p.count, p.timeRange)
}

// =============================================================================
// AliCloud IQS UnifiedSearch API（复用 clawbot runIqsUnifiedSearch）
//
// IQS 使用 ROA（RESTful Open API）风格，非 RPC：
//   URL:    https://iqs.cn-zhangjiakou.aliyuncs.com/linked-retrieval/.../v1/iqs/search/unified
//   Method: POST
//   Body:   JSON
//   Auth:   ROA 签名 → Authorization: acs <AccessKeyId>:<Signature>
// =============================================================================

const iqsPath = "/linked-retrieval/linked-retrieval-entry/v1/iqs/search/unified"

// iqsResponse IQS UnifiedSearch API 响应结构（ROA 风格，无 body 包裹层）。
type iqsResponse struct {
	PageItems []iqsSearchItem `json:"pageItems"`
	RequestID string          `json:"requestId"`
}

type iqsSearchItem struct {
	Title       string  `json:"title"`
	Snippet     string  `json:"snippet"`
	MainText    string  `json:"mainText"`
	Link        string  `json:"link"`
	RerankScore float64 `json:"rerankScore"`
}

func (t *WebSearchTool) searchIQS(ctx context.Context, p *searchParams) ([]searchResult, error) {
	// 构造 JSON 请求体（与 clawbot UnifiedSearchInput 完全一致）
	reqBody := map[string]interface{}{
		"query":      p.query,
		"engineType": "GenericAdvanced",
		"timeRange":  p.timeRange,
		"contents": map[string]interface{}{
			"summary":     true,
			"rerankScore": true,
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	apiURL := fmt.Sprintf("https://%s%s", iqsHost, iqsPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("创建 IQS 请求失败: %w", err)
	}

	// ROA 风格请求头
	dateGMT := time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT")
	contentType := "application/json;charset=UTF-8"

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Date", dateGMT)
	req.Header.Set("User-Agent", "SmartAssistant/1.0 (clawbot-haier-iqs)")
	req.Header.Set("x-acs-version", iqsVersion)

	// 计算 ROA 签名并写入 Authorization 头
	signature := aliyunROASignature(
		http.MethodPost,
		"application/json",
		"",            // Content-MD5（无请求体摘要）
		contentType,
		dateGMT,
		[]roaHeader{ // x-acs-* headers（sorted）
			{key: "x-acs-version", value: iqsVersion},
		},
		iqsPath, // canonicalized resource
		t.accessKeySecret,
	)
	req.Header.Set("Authorization", "acs "+t.accessKeyID+":"+signature)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("IQS 搜索请求超时（%d秒），请稍后重试", p.timeoutSeconds)
		}
		return nil, fmt.Errorf("IQS 搜索请求失败（网络错误）: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResultBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("读取 IQS 响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		detail := string(respBody)
		if len(detail) > int(maxErrorBodyBytes) {
			detail = detail[:maxErrorBodyBytes]
		}
		return nil, fmt.Errorf("IQS API error (HTTP %d): %s", resp.StatusCode, detail)
	}

	var iqsResp iqsResponse
	if err := json.Unmarshal(respBody, &iqsResp); err != nil {
		return nil, fmt.Errorf("解析 IQS 响应失败: %w（原始响应前200字符: %s）", err,
			string(respBody[:min(len(respBody), 200)]))
	}

	// 映射到 searchResult（与 clawbot executeHaierWebSearch 一致）
	var results []searchResult
	for _, item := range iqsResp.PageItems {
		results = append(results, searchResult{
			Title:       item.Title,
			URL:         item.Link,
			Snippet:     item.Snippet,
			Description: item.Snippet,
			Score:       item.RerankScore,
			SiteName:    resolveSiteName(item.Link),
		})
	}

	if len(results) > p.count {
		results = results[:p.count]
	}

	return results, nil
}

// resolveSiteName 从 URL 中提取站点名（复用 clawbot resolveSiteName）。
func resolveSiteName(link string) string {
	if link == "" {
		return ""
	}
	u, err := url.Parse(link)
	if err != nil {
		return ""
	}
	host := u.Hostname()
	// 移除 www. 前缀
	host = strings.TrimPrefix(host, "www.")
	return host
}

// =============================================================================
// AliCloud OpenAPI ROA 签名（Authorization: acs AccessKeyId:Signature）
//
// 参照: https://help.aliyun.com/document_detail/315525.html
//
// StringToSign =
//
//	HTTPMethod + "\n" +
//	Accept + "\n" +
//	Content-MD5 + "\n" +
//	Content-Type + "\n" +
//	Date + "\n" +
//	CanonicalizedHeaders +
//	CanonicalizedResource
//
// Signature = base64(hmac-sha1(StringToSign, AccessKeySecret))
// =============================================================================

// roaHeader 表示一个 x-acs-* 请求头。
type roaHeader struct {
	key   string
	value string
}

// aliyunROASignature 计算阿里云 OpenAPI ROA 风格签名。
func aliyunROASignature(
	method, accept, contentMD5, contentType, date string,
	acsHeaders []roaHeader,
	resourcePath string,
	accessKeySecret string,
) string {
	// CanonicalizedHeaders: sorted x-acs-* headers, lowercase key:value, each terminated by \n
	var canonicalizedHeaders strings.Builder
	for _, h := range acsHeaders {
		canonicalizedHeaders.WriteString(strings.ToLower(h.key))
		canonicalizedHeaders.WriteString(":")
		canonicalizedHeaders.WriteString(strings.TrimSpace(h.value))
		canonicalizedHeaders.WriteString("\n")
	}

	// StringToSign
	stringToSign := method + "\n" +
		accept + "\n" +
		contentMD5 + "\n" +
		contentType + "\n" +
		date + "\n" +
		canonicalizedHeaders.String() +
		resourcePath

	// HMAC-SHA1
	mac := hmac.New(sha1.New, []byte(accessKeySecret))
	mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// =============================================================================
// 输出格式化
// =============================================================================

func formatSearchOutput(payload map[string]interface{}) string {
	var sb strings.Builder

	query, _ := payload["query"].(string)
	provider, _ := payload["provider"].(string)
	count, _ := payload["count"].(float64)
	tookMs, _ := payload["tookMs"].(float64)
	cached, _ := payload["cached"].(bool)

	sb.WriteString(fmt.Sprintf("搜索「%s」结果（通过 %s）：\n\n", query, provider))

	results, ok := payload["results"].([]searchResult)
	if !ok {
		if raw, ok := payload["results"].([]interface{}); ok {
			for _, r := range raw {
				if m, ok := r.(map[string]interface{}); ok {
					title, _ := m["title"].(string)
					urlStr, _ := m["url"].(string)
					snippet, _ := m["snippet"].(string)
					description, _ := m["description"].(string)
					if description != "" && snippet == "" {
						snippet = description
					}
					score, _ := m["score"].(float64)
					siteName, _ := m["siteName"].(string)
					results = append(results, searchResult{
						Title:    title,
						URL:      urlStr,
						Snippet:  snippet,
						Score:    score,
						SiteName: siteName,
					})
				}
			}
		}
	}

	if len(results) == 0 {
		sb.WriteString("未找到相关搜索结果。\n")
	} else {
		for i, r := range results {
			sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, r.Title))
			if r.Snippet != "" {
				sb.WriteString(fmt.Sprintf("   %s\n", r.Snippet))
			}
			if r.URL != "" {
				sb.WriteString(fmt.Sprintf("   链接: %s\n", r.URL))
			}
			if r.SiteName != "" {
				sb.WriteString(fmt.Sprintf("   来源: %s", r.SiteName))
				if r.Score > 0 {
					sb.WriteString(fmt.Sprintf(" · 相关度: %.2f", r.Score))
				}
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}
	}

	meta := fmt.Sprintf("---\n%d 条结果 · %s · %dms", int(count), provider, int(tookMs))
	if cached {
		meta += " (cached)"
	}
	sb.WriteString(meta + "\n")

	return sb.String()
}

// FormatSearchResultJSON 将搜索工具输出格式化为 JSON（用于模型的 tool_result）。
func FormatSearchResultJSON(results []searchResult) string {
	data, _ := json.Marshal(map[string]interface{}{
		"results": results,
		"count":   len(results),
	})
	return string(data)
}

// readLimitedBody 读取响应体的前 n 字节用于错误信息。
func readLimitedBody(r io.Reader, maxBytes int64) string {
	data, err := io.ReadAll(io.LimitReader(r, maxBytes))
	if err != nil {
		return "(failed to read response body)"
	}
	return string(data)
}

// compile-time interface check
var _ Tool = (*WebSearchTool)(nil)