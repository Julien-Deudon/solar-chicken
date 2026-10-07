package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// TimeOfDay représente une heure locale au format HH:MM (colonne texte « HH:MM »).
type TimeOfDay struct {
	Hour   int
	Minute int
}

// ParseTimeOfDay lit "HH:MM" ou "HH:MM:SS".
func ParseTimeOfDay(s string) (*TimeOfDay, error) {
	var h, m int
	if _, err := fmt.Sscanf(strings.TrimSpace(s), "%d:%d", &h, &m); err != nil {
		return nil, fmt.Errorf("heure invalide %q (format attendu HH:MM)", s)
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return nil, fmt.Errorf("heure invalide %q", s)
	}
	return &TimeOfDay{Hour: h, Minute: m}, nil
}

// On place l'heure sur le jour donné, dans le fuseau loc.
func (t TimeOfDay) On(day time.Time, loc *time.Location) time.Time {
	d := day.In(loc)
	return time.Date(d.Year(), d.Month(), d.Day(), t.Hour, t.Minute, 0, 0, loc)
}

func (t TimeOfDay) String() string { return fmt.Sprintf("%02d:%02d", t.Hour, t.Minute) }

// Scan implémente sql.Scanner.
func (t *TimeOfDay) Scan(value interface{}) error {
	switch v := value.(type) {
	case nil:
		return nil
	case []byte:
		_, err := fmt.Sscanf(string(v), "%d:%d", &t.Hour, &t.Minute)
		return err
	case string:
		_, err := fmt.Sscanf(v, "%d:%d", &t.Hour, &t.Minute)
		return err
	case time.Time:
		t.Hour, t.Minute = v.Hour(), v.Minute()
		return nil
	}
	return fmt.Errorf("cannot scan type %T into TimeOfDay", value)
}

// Value implémente driver.Valuer.
func (t TimeOfDay) Value() (driver.Value, error) {
	return t.String(), nil
}

func (t TimeOfDay) MarshalJSON() ([]byte, error) { return json.Marshal(t.String()) }

func (t *TimeOfDay) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	p, err := ParseTimeOfDay(s)
	if err != nil {
		return err
	}
	*t = *p
	return nil
}
