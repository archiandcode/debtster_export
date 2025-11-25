package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"debtster-export/internal/clients"
)

type ExportService struct {
	redis       *clients.RedisClient
	cachePrefix string
}

func NewExportService(redis *clients.RedisClient, cachePrefix string) *ExportService {
	return &ExportService{
		redis:       redis,
		cachePrefix: cachePrefix,
	}
}

func (s *ExportService) GetExports(ctx context.Context, userID int64) ([]interface{}, error) {
	if s.redis == nil {
		return nil, errors.New("redis client not configured")
	}

	keys, err := s.redis.SMembers(ctx, "export_ids")
	if err != nil {
		return nil, fmt.Errorf("failed to get export keys: %w", err)
	}

	var exports []interface{}

	var statuses []ExportStatus
	for _, key := range keys {
		data, err := s.redis.Get(ctx, key)
		if err != nil {
			continue
		}

		var status ExportStatus
		if err := json.Unmarshal([]byte(data), &status); err != nil {
			continue
		}

		if status.UserID == userID {
			statuses = append(statuses, status)
		}
	}

	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].Created.After(statuses[j].Created)
	})

	for _, status := range statuses {
		exportMap := map[string]interface{}{
			"key":        status.Key,
			"type":       status.Type,
			"user_id":    status.UserID,
			"progress":   status.Progress,
			"file_url":   status.FileURL,
			"filters":    status.Filters,
			"created_at": humanizeRuAgo(status.Created),
		}
		exports = append(exports, exportMap)
	}

	return exports, nil
}

func humanizeRuAgo(t time.Time) string {
	now := time.Now()
	if t.After(now) {
		return "только что"
	}

	diff := now.Sub(t)
	minutes := int(diff.Minutes())
	if minutes < 1 {
		return "только что"
	}
	if minutes < 60 {
		return fmt.Sprintf("%d %s назад", minutes, ruPlural(minutes, "минута", "минуты", "минут"))
	}
	hours := minutes / 60
	if hours < 24 {
		return fmt.Sprintf("%d %s назад", hours, ruPlural(hours, "час", "часа", "часов"))
	}
	days := hours / 24
	if days < 30 {
		return fmt.Sprintf("%d %s назад", days, ruPlural(days, "день", "дня", "дней"))
	}
	return t.Format("02.01.2006 15:04")
}

func ruPlural(n int, one, few, many string) string {
	n = n % 100
	if n >= 11 && n <= 14 {
		return many
	}
	n = n % 10
	switch n {
	case 1:
		return one
	case 2, 3, 4:
		return few
	default:
		return many
	}
}

func (s *ExportService) GetExport(ctx context.Context, exportID string, userID int64) (interface{}, error) {
	if s.redis == nil {
		return nil, errors.New("redis client not configured")
	}

	data, err := s.redis.Get(ctx, exportID)
	if err != nil {
		return nil, errors.New("export not found")
	}

	var status ExportStatus
	if err := json.Unmarshal([]byte(data), &status); err != nil {
		return nil, fmt.Errorf("failed to parse export status: %w", err)
	}

	if status.UserID != userID {
		return nil, errors.New("export not found")
	}

	exportMap := map[string]interface{}{
		"key":        status.Key,
		"type":       status.Type,
		"user_id":    status.UserID,
		"progress":   status.Progress,
		"file_url":   status.FileURL,
		"filters":    status.Filters,
		"created_at": humanizeRuAgo(status.Created),
	}

	return exportMap, nil
}
