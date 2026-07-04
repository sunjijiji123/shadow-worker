package storage

import (
	"context"
	"fmt"
	"time"
)

// WorklogSegmentOut 是返回给 agent 的时段。
type WorklogSegmentOut struct {
	Start    string `json:"start"`
	End      string `json:"end"`
	App      string `json:"app"`
	Category string `json:"category"`
	Minutes  int    `json:"minutes"`
	Summary  string `json:"summary,omitempty"`
	Hint     string `json:"hint,omitempty"`
}

// WorklogEventOut 是返回给 agent 的事件。
type WorklogEventOut struct {
	Time string `json:"time"`
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// WorklogResult 是 get_worklog 的返回结构。
type WorklogResult struct {
	Date               string              `json:"date"`
	TotalActiveMinutes int                 `json:"total_active_minutes"`
	TotalSegments      int                 `json:"total_segments"`     // 过滤后段总数（分页前），供 agent 判断是否翻页
	TotalEvents        int                 `json:"total_events"`       // 事件总数（事件不分页，此处仅供参考）
	ReturnedSegments   int                 `json:"returned_segments"`  // 本次返回的段数（分页后的子集大小）
	HasMore            bool                `json:"has_more"`           // 是否还有下一页段
	Segments           []WorklogSegmentOut `json:"segments"`           // 分页后的段子集
	Events             []WorklogEventOut   `json:"events"`             // 事件全量（与段是两个维度，不参与分页）
}

// SummaryGroup 是 get_summary 的分组项。
type SummaryGroup struct {
	Key     string   `json:"key"`
	Minutes int      `json:"minutes"`
	Apps    []string `json:"apps"`
}

// SummaryResult 是 get_summary 的返回结构。
type SummaryResult struct {
	Date               string         `json:"date"`
	TotalActiveMinutes int            `json:"total_active_minutes"`
	Groups             []SummaryGroup `json:"groups"`
}

// SearchEventOut 是 search_events 的返回项。
type SearchEventOut struct {
	Date string `json:"date"`
	Time string `json:"time"`
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// SearchEventsResult 是 search_events 的返回结构。
type SearchEventsResult struct {
	Results []SearchEventOut `json:"results"`
}

// AppOut 是 list_apps 的返回项。
type AppOut struct {
	Path         string `json:"path"`
	Name         string `json:"name"`
	Category     string `json:"category"`
	TodayMinutes int    `json:"today_minutes"`
}

// ListAppsResult 是 list_apps 的返回结构。
type ListAppsResult struct {
	Apps []AppOut `json:"apps"`
}

// parseDate 解析 YYYY-MM-DD 为当天的起止时间（本地时区）。
// 使用本地时区切日，避免 UTC 切日导致凌晨 0-8 点的事件归属错位（坑 #44）。
func parseDate(date string) (time.Time, time.Time, error) {
	day, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("日期格式错误，应为 YYYY-MM-DD: %w", err)
	}
	return day, day.Add(24 * time.Hour), nil
}

// formatHM 格式化为本地时区的 HH:MM。
func formatHM(t time.Time) string {
	return t.Local().Format("15:04")
}

// GetWorklog 查询指定日期的工作日志。limit/offset 对 segments 分页。
// eventTypes 过滤返回的事件类型；为 nil/空时不过滤（全量）。
// 注意：业务默认值（如"只返 voice"）由调用方 server 层决定，storage 层只做纯过滤。
func (db *DB) GetWorklog(ctx context.Context, date, categoryFilter, appFilter string, limit, offset int, eventTypes []string) (*WorklogResult, error) {
	start, end, err := parseDate(date)
	if err != nil {
		return nil, err
	}

	segs, err := db.ListActivitySegments(start, end)
	if err != nil {
		return nil, fmt.Errorf("查询活动段失败: %w", err)
	}

	events, err := db.ListEvents(start, end)
	if err != nil {
		return nil, fmt.Errorf("查询事件失败: %w", err)
	}

	// 先按 category/app 过滤 + 跳过 idle，得到完整的过滤后段切片。
	// 注意：TotalActiveMinutes / TotalSegments 都基于"过滤后的全集"统计（分页前），
	// 这样 agent 看到的总时长/总段数是准确的，不会因分页而变小。
	filtered := make([]WorklogSegmentOut, 0, len(segs))
	totalActive := 0
	for _, seg := range segs {
		if categoryFilter != "" && seg.Category != categoryFilter {
			continue
		}
		if appFilter != "" && seg.AppName != appFilter {
			continue
		}
		if seg.State == "idle" {
			continue
		}

		minutes := int(seg.EndTS.Sub(seg.StartTS).Minutes())
		if minutes < 1 {
			minutes = 1
		}

		so := WorklogSegmentOut{
			Start:    formatHM(seg.StartTS),
			End:      formatHM(seg.EndTS),
			App:      seg.AppName,
			Category: seg.Category,
			Minutes:  minutes,
		}
		if seg.Category == "coding" {
			so.Hint = "use code-worklog skill"
		}
		filtered = append(filtered, so)
		totalActive += minutes
	}

	// events 转换（不分页，但按 eventTypes 过滤）。TotalEvents 统计过滤后的数量。
	// 注意：默认情况下调用方只传 voice（vlm_summary 高频碎、单日可达数百条占大量 token，
	// 对日报价值低），需要 VLM 摘要时调用方显式传 ["vlm_summary"] 或用 search_events。
	typeSet := map[string]bool{}
	for _, t := range eventTypes {
		typeSet[t] = true
	}
	evOut := make([]WorklogEventOut, 0, len(events))
	for _, ev := range events {
		if len(typeSet) > 0 && !typeSet[string(ev.Type)] {
			continue
		}
		evOut = append(evOut, WorklogEventOut{
			Time: formatHM(ev.TS),
			Type: string(ev.Type),
			Text: ev.Content,
		})
	}

	// segments 分页：限制单页大小，防止日志量过大撑爆 LLM 上下文（坑 #55）。
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	total := len(filtered)
	endIdx := offset + limit
	if endIdx > total {
		endIdx = total
	}
	if offset > total { // offset 越界 → 返回空切片而非 panic
		offset = total
	}
	paged := filtered[offset:endIdx]

	return &WorklogResult{
		Date:               date,
		TotalActiveMinutes: totalActive,
		TotalSegments:      total,
		TotalEvents:        len(evOut),
		ReturnedSegments:   len(paged),
		HasMore:            endIdx < total,
		Segments:           paged,
		Events:             evOut,
	}, nil
}

// GetSummary 查询指定日期的聚合统计。
func (db *DB) GetSummary(ctx context.Context, date, groupBy string) (*SummaryResult, error) {
	start, end, err := parseDate(date)
	if err != nil {
		return nil, err
	}
	if groupBy == "" {
		groupBy = "category"
	}

	segs, err := db.ListActivitySegments(start, end)
	if err != nil {
		return nil, fmt.Errorf("查询活动段失败: %w", err)
	}

	groups := make(map[string]*SummaryGroup)
	total := 0
	for _, seg := range segs {
		if seg.State == "idle" {
			continue
		}
		minutes := int(seg.EndTS.Sub(seg.StartTS).Minutes())
		if minutes < 1 {
			minutes = 1
		}
		total += minutes

		key := seg.Category
		if groupBy == "app" {
			key = seg.AppName
		}

		g, ok := groups[key]
		if !ok {
			g = &SummaryGroup{Key: key}
			groups[key] = g
		}
		g.Minutes += minutes
		if !contains(g.Apps, seg.AppName) {
			g.Apps = append(g.Apps, seg.AppName)
		}
	}

	out := &SummaryResult{
		Date:               date,
		TotalActiveMinutes: total,
		Groups:             []SummaryGroup{},
	}
	for _, g := range groups {
		out.Groups = append(out.Groups, *g)
	}
	return out, nil
}

// CategoryStat 是摘要桶内某类别的分钟数。
type CategoryStat struct {
	Category string `json:"category"`
	Minutes  int    `json:"minutes"`
}

// AppStat 是摘要桶内某应用的分钟数。
type AppStat struct {
	App     string `json:"app"`
	Minutes int    `json:"minutes"`
}

// WorklogSummaryBucket 是一个时段桶（上午/下午/晚上之一）。
type WorklogSummaryBucket struct {
	Period     string         `json:"period"`     // "morning" / "afternoon" / "evening"
	Range      string         `json:"range"`      // "00:00-12:00" 等，便于 agent 理解
	Minutes    int            `json:"minutes"`    // 该时段总活跃分钟
	Categories []CategoryStat `json:"categories"` // 该时段各类别分钟数（按分钟降序）
	TopApps    []AppStat      `json:"top_apps"`   // 该时段 top3 应用（按分钟降序）
}

// WorklogSummaryResult 是 get_worklog_summary 的返回结构：三时段压缩视图。
// 面向"先看全局再下钻"场景，token 开销小（~1KB），与 get_summary 的纯统计聚合
// 定位不同（后者不分时段）。
type WorklogSummaryResult struct {
	Date               string                 `json:"date"`
	TotalActiveMinutes int                    `json:"total_active_minutes"`
	Buckets            []WorklogSummaryBucket `json:"buckets"` // 最多 3 个，无活动的时段不出现
}

// GetWorklogSummary 查询指定日期工作日志的三时段（上午/下午/晚上）压缩摘要。
//
// 时段划分基于段 start_ts 的【本地小时】（morning[0,12) / afternoon[12,18) / evening[18,24)）。
// 注意：parseDate 用 UTC 切日（坑 #44 已知问题），但时段桶判断用 StartTS.Local().Hour()，
// 不受 parseDate 的 UTC 影响——"上午/下午"本就是本地作息概念。单机部署后端与客户端同机器、
// 同时区，本地小时即用户体感时段。
//
// 跨午夜段（如熬夜到次日 1 点）：当前按 start_ts 落桶，end 不参与（段已由采集层在本地
// 午夜虚拟切分，见 clipSegmentsToDay，跨天部分不在此日的查询结果里）。
func (db *DB) GetWorklogSummary(ctx context.Context, date string) (*WorklogSummaryResult, error) {
	start, end, err := parseDate(date)
	if err != nil {
		return nil, err
	}

	segs, err := db.ListActivitySegments(start, end)
	if err != nil {
		return nil, fmt.Errorf("查询活动段失败: %w", err)
	}

	// 桶索引：0=morning, 1=afternoon, 2=evening。初始全 nil，
	// 首次落入某桶时懒初始化（设置 period/rng）；无活动的桶保持 nil 不输出。
	buckets := [3]*summaryAccumulator{}
	total := 0
	for _, seg := range segs {
		if seg.State == "idle" {
			continue
		}
		minutes := int(seg.EndTS.Sub(seg.StartTS).Minutes())
		if minutes < 1 {
			minutes = 1
		}
		total += minutes

		idx := bucketIndex(seg.StartTS.Local().Hour())
		acc := buckets[idx]
		if acc == nil {
			acc = &summaryAccumulator{period: bucketPeriod(idx), rng: bucketRange(idx)}
			buckets[idx] = acc
		}
		acc.minutes += minutes
		acc.addCategory(seg.Category, minutes)
		acc.addApp(seg.AppName, minutes)
	}

	out := &WorklogSummaryResult{
		Date:               date,
		TotalActiveMinutes: total,
		Buckets:            []WorklogSummaryBucket{},
	}
	for _, acc := range buckets {
		if acc == nil || acc.minutes == 0 {
			continue // 无活动的时段不输出，省 token
		}
		out.Buckets = append(out.Buckets, acc.toBucket())
	}
	return out, nil
}

// summaryAccumulator 是桶聚合的中间结构（未排序、用 map 累加）。
type summaryAccumulator struct {
	period    string
	rng       string
	minutes   int
	cats      map[string]int
	apps      map[string]int
}

func (a *summaryAccumulator) addCategory(cat string, minutes int) {
	if a.cats == nil {
		a.cats = map[string]int{}
	}
	a.cats[cat] += minutes
}

func (a *summaryAccumulator) addApp(app string, minutes int) {
	if a.apps == nil {
		a.apps = map[string]int{}
	}
	a.apps[app] += minutes
}

// toBucket 把累加器转成最终输出桶：类别按分钟降序全列，应用取 top3 降序。
func (a *summaryAccumulator) toBucket() WorklogSummaryBucket {
	b := WorklogSummaryBucket{
		Period:     a.period,
		Range:      a.rng,
		Minutes:    a.minutes,
		Categories: []CategoryStat{},
		TopApps:    []AppStat{},
	}
	for cat, m := range a.cats {
		b.Categories = append(b.Categories, CategoryStat{Category: cat, Minutes: m})
	}
	sortStats(b.Categories) // CategoryStat 按分钟降序
	for app, m := range a.apps {
		b.TopApps = append(b.TopApps, AppStat{App: app, Minutes: m})
	}
	sortStats(b.TopApps) // AppStat 按分钟降序
	if len(b.TopApps) > 3 {
		b.TopApps = b.TopApps[:3] // top3
	}
	return b
}

// bucketIndex 把本地小时映射到桶索引：0=morning[0,12), 1=afternoon[12,18), 2=evening[18,24)。
func bucketIndex(hour int) int {
	switch {
	case hour < 12:
		return 0
	case hour < 18:
		return 1
	default:
		return 2
	}
}

func bucketPeriod(idx int) string {
	return [...]string{"morning", "afternoon", "evening"}[idx]
}

func bucketRange(idx int) string {
	return [...]string{"00:00-12:00", "12:00-18:00", "18:00-24:00"}[idx]
}

// sortStats 按 Minutes 降序排序（稳定）。用于摘要桶内类别/应用排序。
// 用冒泡而非 sort.Slice 是为了保持 worklog.go 零额外 import 的简洁性，
// 且桶内元素很少（类别 ≤5、应用几十个），性能无差异。
func sortStats[T minuteKeyer](s []T) {
	n := len(s)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if s[j].minutes() > s[i].minutes() {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

type minuteKeyer interface {
	minutes() int
}

func (c CategoryStat) minutes() int { return c.Minutes }
func (a AppStat) minutes() int      { return a.Minutes }

// SearchEventsByQuery 按关键词搜索事件。
func (db *DB) SearchEventsByQuery(ctx context.Context, query, date string, eventType string) (*SearchEventsResult, error) {
	var start, end time.Time
	if date != "" {
		var err error
		start, end, err = parseDate(date)
		if err != nil {
			return nil, err
		}
	} else {
		now := time.Now()
		start = StartOfLocalDay(now)
		end = start.Add(24 * time.Hour)
	}

	var events []Event
	var err error
	if query != "" {
		events, err = db.SearchEvents(query, start, end)
	} else {
		events, err = db.ListEvents(start, end)
	}
	if err != nil {
		return nil, fmt.Errorf("搜索事件失败: %w", err)
	}

	out := &SearchEventsResult{Results: []SearchEventOut{}}
	for _, ev := range events {
		if eventType != "" && string(ev.Type) != eventType {
			continue
		}
		out.Results = append(out.Results, SearchEventOut{
			Date: ev.TS.UTC().Format("2006-01-02"),
			Time: formatHM(ev.TS),
			Type: string(ev.Type),
			Text: ev.Content,
		})
	}
	return out, nil
}

// ListAppsWithTodayMinutes 列出白名单应用及今日时长。
func (db *DB) ListAppsWithTodayMinutes(ctx context.Context) (*ListAppsResult, error) {
	apps, err := db.ListAppCategories()
	if err != nil {
		return nil, fmt.Errorf("列出白名单失败: %w", err)
	}

	now := time.Now()
	start := StartOfLocalDay(now)
	end := start.Add(24 * time.Hour)

	segs, err := db.ListActivitySegments(start, end)
	if err != nil {
		return nil, fmt.Errorf("查询今日活动段失败: %w", err)
	}

	minutesByApp := make(map[string]int)
	for _, seg := range segs {
		if seg.State == "idle" {
			continue
		}
		m := int(seg.EndTS.Sub(seg.StartTS).Minutes())
		if m < 1 {
			m = 1
		}
		minutesByApp[seg.AppPath] += m
	}

	out := &ListAppsResult{Apps: []AppOut{}}
	for _, app := range apps {
		out.Apps = append(out.Apps, AppOut{
			Path:         app.Path,
			Name:         app.Name,
			Category:     app.Category,
			TodayMinutes: minutesByApp[app.Path],
		})
	}
	return out, nil
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
