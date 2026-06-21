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
	Segments           []WorklogSegmentOut `json:"segments"`
	Events             []WorklogEventOut   `json:"events"`
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

// parseDate 解析 YYYY-MM-DD 为当天的起止时间（UTC）。
func parseDate(date string) (time.Time, time.Time, error) {
	day, err := time.Parse("2006-01-02", date)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("日期格式错误，应为 YYYY-MM-DD: %w", err)
	}
	day = day.UTC()
	return day, day.Add(24 * time.Hour), nil
}

// formatHM 格式化为 HH:MM。
func formatHM(t time.Time) string {
	return t.UTC().Format("15:04")
}

// GetWorklog 查询指定日期的工作日志。
func (db *DB) GetWorklog(ctx context.Context, date, categoryFilter, appFilter string) (*WorklogResult, error) {
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

	out := &WorklogResult{
		Date:     date,
		Segments: []WorklogSegmentOut{},
		Events:   []WorklogEventOut{},
	}
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
		out.Segments = append(out.Segments, so)
		out.TotalActiveMinutes += minutes
	}

	for _, ev := range events {
		out.Events = append(out.Events, WorklogEventOut{
			Time: formatHM(ev.TS),
			Type: string(ev.Type),
			Text: ev.Content,
		})
	}

	return out, nil
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
		now := time.Now().UTC()
		start = now.Truncate(24 * time.Hour)
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

	now := time.Now().UTC()
	start := now.Truncate(24 * time.Hour)
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
