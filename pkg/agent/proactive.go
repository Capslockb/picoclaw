package agent

import (
	"fmt"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/cron"
)

// ProactiveService manages automated background tasks for communication processing.
type ProactiveService struct {
	cron *cron.CronService
	cfg  config.ProactiveConfig
}

// NewProactiveService creates a new ProactiveService.
func NewProactiveService(cron *cron.CronService, cfg config.ProactiveConfig) *ProactiveService {
	return &ProactiveService{
		cron: cron,
		cfg:  cfg,
	}
}

// Start registers the automated jobs in the cron service.
func (s *ProactiveService) Start() error {
	if !s.cfg.Enabled {
		return nil
	}

	// 1. Job for syncing messaging history (WhatsApp/Gmail)
	if s.cfg.SyncIntervalMinutes > 0 {
		syncIntervalMS := int64(s.cfg.SyncIntervalMinutes) * 60 * 1000
		
		// Note: We use a system-internal channel name to avoid cluttering user chat
		_, err := s.cron.AddJob(
			"Auto Sync Communications",
			cron.CronSchedule{Kind: "every", EveryMS: &syncIntervalMS},
			"Automatically sync my latest WhatsApp and Gmail communications to contextual memory.",
			false, // Process via agent
			"system",
			"proactive_sync",
		)
		if err != nil {
			return fmt.Errorf("failed to register auto-sync job: %w", err)
		}
	}

	// 2. Job for processing insights and auto-updating Calendar/TODO
	if s.cfg.ProcessIntervalMinutes > 0 {
		processIntervalMS := int64(s.cfg.ProcessIntervalMinutes) * 60 * 1000
		
		prompt := "PROACTIVE SYSTEM TURN: Analyze recent communications in COMMUNICATIONS.md. Identify any new calendar events or tasks. Update my Google Calendar and TODO list if necessary. Be silent if no actions are taken."
		
		_, err := s.cron.AddJob(
			"Proactive Action Extraction",
			cron.CronSchedule{Kind: "every", EveryMS: &processIntervalMS},
			prompt,
			false, // Process via agent
			"system",
			"proactive_process",
		)
		if err != nil {
			return fmt.Errorf("failed to register proactive processing job: %w", err)
		}
	}

	return nil
}
