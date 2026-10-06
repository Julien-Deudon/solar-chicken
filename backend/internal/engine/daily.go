package engine

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/julien-deudon/solar-chicken/backend/internal/models"
	"github.com/julien-deudon/solar-chicken/backend/internal/planner"
)

// dailyReports envoie une fois par jour, entre 6 h et midi, le planning du jour de chaque poulailler.
func (e *Engine) dailyReports(ctx context.Context) {
	if e.Mode != ModeLive {
		return
	}
	coops, err := e.loadCoops()
	if err != nil {
		return
	}
	for i := range coops {
		coop := &coops[i]
		loc, err := coop.Location()
		if err != nil {
			continue
		}
		local := e.now().In(loc)
		today := planner.DayKey(local, loc)
		if local.Hour() < e.Timing.DailyReportHour || coop.LastDailyReportDay == today {
			continue
		}
		if local.Hour() < 12 {
			msg, err := e.DailySummary(coop, today)
			if err != nil {
				log.Printf("❌ Planning du jour %s : %v", coop.Name, err)
				continue
			}
			e.notify(ctx, coop, NotifyDaily, msg)
		}
		e.DB.Model(coop).Update("last_daily_report_day", today)
	}
}

// DailySummary construit le message du planning du jour.
func (e *Engine) DailySummary(coop *models.Coop, day string) (string, error) {
	loc, err := coop.Location()
	if err != nil {
		return "", err
	}
	plan, err := planner.PlanDay(coop, coop.Devices, e.now().In(loc))
	if err != nil {
		return "", err
	}
	var events []models.PlannedEvent
	if err := e.DB.Where("coop_id = ? AND day = ? AND status <> ?", coop.ID, day, models.StatusSkipped).Order("due_at").Find(&events).Error; err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, t(coop, "🌅 <b>Planning du jour</b> · %s\n☀️ Lever %s · Coucher %s\n", "🌅 <b>Today's schedule</b> · %s\n☀️ Sunrise %s · Sunset %s\n"), coop.Name, hhmm(coop, plan.Sunrise), hhmm(coop, plan.Sunset))
	for _, dev := range coop.Devices {
		if !dev.Automated() {
			continue
		}
		var parts []string
		for _, ev := range events {
			if ev.DeviceID != dev.ID {
				continue
			}
			icon := map[models.EventAction]string{
				models.EventOpen: "🔓", models.EventClose: "🔒",
				models.EventLightBeforeOpen: "💡", models.EventLightBeforeClose: "💡",
				models.EventLightAfterOpen: "🌙", models.EventLightAfterClose: "🌙",
			}[ev.Action]
			parts = append(parts, icon+" "+hhmm(coop, ev.DueAt))
		}
		if len(parts) == 0 {
			if msg, ok := plan.Errors[dev.ID]; ok {
				parts = append(parts, "⚠️ "+msg)
			} else {
				continue
			}
		}
		icon := "🐔"
		if dev.Role == models.RoleNestBox {
			icon = "🥚"
		}
		suffix := ""
		if dev.Strategy == models.StrategyOnboard {
			suffix = t(coop, " <i>(horaires dans le boîtier)</i>", " <i>(times stored in the unit)</i>")
		}
		fmt.Fprintf(&b, "\n%s <b>%s</b>%s\n  %s\n", icon, dev.Name, suffix, strings.Join(parts, " · "))
	}
	b.WriteString(t(coop, "\nBonne journée ! ☀️", "\nHave a nice day! ☀️"))
	return b.String(), nil
}
