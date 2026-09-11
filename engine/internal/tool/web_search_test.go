package tool

import (
	"testing"
	"time"
)

// =============================================================================
// AliCloud ROA 签名测试
// =============================================================================

func TestAliyunROASignature(t *testing.T) {
	roaHeaders := []roaHeader{
		{key: "x-acs-version", value: "2024-11-11"},
	}

	sig := aliyunROASignature(
		"POST",
		"application/json",
		"",
		"application/json;charset=UTF-8",
		"Mon, 15 Jan 2024 10:00:00 GMT",
		roaHeaders,
		"/linked-retrieval/linked-retrieval-entry/v1/iqs/search/unified",
		"test-secret",
	)

	if sig == "" {
		t.Error("signature should not be empty")
	}

	// 签名可重现
	sig2 := aliyunROASignature(
		"POST",
		"application/json",
		"",
		"application/json;charset=UTF-8",
		"Mon, 15 Jan 2024 10:00:00 GMT",
		roaHeaders,
		"/linked-retrieval/linked-retrieval-entry/v1/iqs/search/unified",
		"test-secret",
	)
	if sig != sig2 {
		t.Error("signature should be deterministic for same inputs")
	}

	// 不同 secret 产生不同签名
	sig3 := aliyunROASignature(
		"POST",
		"application/json",
		"",
		"application/json;charset=UTF-8",
		"Mon, 15 Jan 2024 10:00:00 GMT",
		roaHeaders,
		"/linked-retrieval/linked-retrieval-entry/v1/iqs/search/unified",
		"different-secret",
	)
	if sig == sig3 {
		t.Error("different secrets should produce different signatures")
	}

	// 不同 path 产生不同签名
	sig4 := aliyunROASignature(
		"POST",
		"application/json",
		"",
		"application/json;charset=UTF-8",
		"Mon, 15 Jan 2024 10:00:00 GMT",
		roaHeaders,
		"/different/path",
		"test-secret",
	)
	if sig == sig4 {
		t.Error("different paths should produce different signatures")
	}
}

// =============================================================================
// 参数解析测试
// =============================================================================

func TestParseSearchArgs(t *testing.T) {
	// 基本参数
	args := map[string]interface{}{"query": "test query"}
	p, err := parseSearchArgs(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.query != "test query" {
		t.Errorf("query = %q, want %q", p.query, "test query")
	}
	if p.count != defaultSearchCount {
		t.Errorf("count = %d, want %d", p.count, defaultSearchCount)
	}

	// 带 count
	args = map[string]interface{}{"query": "test", "count": float64(3)}
	p, err = parseSearchArgs(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.count != 3 {
		t.Errorf("count = %d, want 3", p.count)
	}

	// count 超过上限
	args = map[string]interface{}{"query": "test", "count": float64(100)}
	p, err = parseSearchArgs(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.count != maxSearchCount {
		t.Errorf("count = %d, want %d (capped at max)", p.count, maxSearchCount)
	}

	// 缺少 query
	args = map[string]interface{}{"count": float64(5)}
	_, err = parseSearchArgs(args)
	if err == nil {
		t.Error("expected error for missing query")
	}

	// 空 query
	args = map[string]interface{}{"query": "  "}
	_, err = parseSearchArgs(args)
	if err == nil {
		t.Error("expected error for empty query")
	}

	// freshness → time_range 映射
	args = map[string]interface{}{"query": "test", "freshness": "day"}
	p, err = parseSearchArgs(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.freshness != "day" {
		t.Errorf("freshness = %q, want %q", p.freshness, "day")
	}
	if p.timeRange != "OneDay" {
		t.Errorf("timeRange = %q, want %q (from freshness=day)", p.timeRange, "OneDay")
	}

	// time_range 直接指定
	args = map[string]interface{}{"query": "test", "time_range": "OneWeek"}
	p, err = parseSearchArgs(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.timeRange != "OneWeek" {
		t.Errorf("timeRange = %q, want %q", p.timeRange, "OneWeek")
	}

	// freshness=month 映射
	args = map[string]interface{}{"query": "test", "freshness": "month"}
	p, err = parseSearchArgs(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.timeRange != "OneMonth" {
		t.Errorf("timeRange = %q, want OneMonth", p.timeRange)
	}

	// freshness=year 映射
	args = map[string]interface{}{"query": "test", "freshness": "year"}
	p, err = parseSearchArgs(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.timeRange != "OneYear" {
		t.Errorf("timeRange = %q, want OneYear", p.timeRange)
	}
}

// =============================================================================
// readPositiveInt 测试
// =============================================================================

func TestReadPositiveInt(t *testing.T) {
	// 默认值
	n := readPositiveInt(map[string]interface{}{}, "missing", 5, 1, 10)
	if n != 5 {
		t.Errorf("got %d, want 5", n)
	}

	// 正常值
	n = readPositiveInt(map[string]interface{}{"x": float64(3)}, "x", 5, 1, 10)
	if n != 3 {
		t.Errorf("got %d, want 3", n)
	}

	// 低于最小值
	n = readPositiveInt(map[string]interface{}{"x": float64(0)}, "x", 5, 1, 10)
	if n != 1 {
		t.Errorf("got %d, want 1 (capped at min)", n)
	}

	// 高于最大值
	n = readPositiveInt(map[string]interface{}{"x": float64(100)}, "x", 5, 1, 10)
	if n != 10 {
		t.Errorf("got %d, want 10 (capped at max)", n)
	}

	// int 类型
	n = readPositiveInt(map[string]interface{}{"x": 7}, "x", 5, 1, 10)
	if n != 7 {
		t.Errorf("got %d, want 7", n)
	}
}

// =============================================================================
// 缓存测试
// =============================================================================

func TestSearchCache(t *testing.T) {
	// 清理缓存
	searchCacheMu.Lock()
	searchCache = make(map[string]cacheEntry)
	searchCacheMu.Unlock()

	key := "test-key"
	val := map[string]interface{}{"foo": "bar"}

	// 缓存未命中
	if cached, ok := readCache(key); ok {
		t.Errorf("unexpected cache hit: %v", cached)
	}

	// 写入缓存
	writeCache(key, val, 10*time.Minute)

	// 缓存命中
	cached, ok := readCache(key)
	if !ok {
		t.Error("expected cache hit")
	}
	if cached["foo"] != "bar" {
		t.Errorf("cached value = %v, want foo=bar", cached)
	}
}

// =============================================================================
// Cache Key 格式测试（IQS 版本）
// =============================================================================

func TestBuildCacheKey(t *testing.T) {
	p := &searchParams{
		query:     "test query",
		count:     5,
		timeRange: "OneDay",
	}
	key := buildCacheKey(p)
	expected := "iqs:test query:5:OneDay"
	if key != expected {
		t.Errorf("cache key = %q, want %q", key, expected)
	}
}

// =============================================================================
// Definition 测试 — 确保 schema 包含核心参数
// =============================================================================

func TestWebSearchToolDefinition(t *testing.T) {
	tool := NewWebSearchTool()
	def := tool.Definition()

	if def.Name != "web_search" {
		t.Errorf("Name = %q, want %q", def.Name, "web_search")
	}

	params := def.Parameters

	// 验证必需字段
	required, ok := params["required"].([]string)
	if !ok {
		t.Fatal("required is not []string")
	}
	if len(required) != 1 || required[0] != "query" {
		t.Errorf("required = %v, want [query]", required)
	}

	// 验证 8 个参数都存在（IQS 核心参数）
	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("properties is not a map")
	}

	expectedParams := []string{
		"query", "count", "time_range",
		"country", "language", "freshness",
		"date_after", "date_before",
	}
	for _, name := range expectedParams {
		if _, exists := props[name]; !exists {
			t.Errorf("missing parameter: %s", name)
		}
	}

	if len(props) != 8 {
		t.Errorf("expected 8 parameters (IQS native), got %d", len(props))
	}
}

// =============================================================================
// SiteName 解析测试
// =============================================================================

func TestResolveSiteName(t *testing.T) {
	tests := []struct {
		link     string
		expected string
	}{
		{"https://www.example.com/page", "example.com"},
		{"https://example.com", "example.com"},
		{"https://blog.example.org/article", "blog.example.org"},
		{"http://sub.domain.co.uk/path", "sub.domain.co.uk"},
		{"", ""},
		{"/relative/path", ""},
	}

	for _, tt := range tests {
		got := resolveSiteName(tt.link)
		if got != tt.expected {
			t.Errorf("resolveSiteName(%q) = %q, want %q", tt.link, got, tt.expected)
		}
	}
}

// =============================================================================
// formatSearchOutput 测试（IQS 格式）
// =============================================================================

func TestFormatSearchOutput(t *testing.T) {
	payload := map[string]interface{}{
		"query":    "test query",
		"provider": "haier_web_search",
		"count":    float64(2),
		"tookMs":   float64(150),
		"results": []searchResult{
			{
				Title:    "Test Result 1",
				URL:      "https://example.com/1",
				Snippet:  "This is a test snippet",
				Score:    0.95,
				SiteName: "example.com",
			},
			{
				Title:    "Test Result 2",
				URL:      "https://example.org/2",
				Snippet:  "Another test snippet",
				Score:    0.85,
				SiteName: "example.org",
			},
		},
	}

	output := formatSearchOutput(payload)

	// 验证关键内容存在
	checks := []string{
		"test query",
		"haier_web_search",
		"Test Result 1",
		"https://example.com/1",
		"This is a test snippet",
		"example.com",
		"0.95",
		"Test Result 2",
		"2 条结果",
		"150ms",
	}
	for _, check := range checks {
		if !containsString(output, check) {
			t.Errorf("output should contain %q, got:\n%s", check, output)
		}
	}
}

func TestFormatSearchOutputEmpty(t *testing.T) {
	payload := map[string]interface{}{
		"query":    "no results",
		"provider": "haier_web_search",
		"count":    float64(0),
		"tookMs":   float64(50),
		"results":  []searchResult{},
	}

	output := formatSearchOutput(payload)
	if !containsString(output, "未找到相关搜索结果") {
		t.Errorf("output should indicate no results, got:\n%s", output)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}