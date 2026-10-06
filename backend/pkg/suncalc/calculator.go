package suncalc

import (
	"time"

	"github.com/sixdouglas/suncalc"
)

// Calculator calcule les heures de lever et coucher du soleil
type Calculator struct {
	Latitude  float64
	Longitude float64
	Timezone  string
}

// SunTimes contient les horaires de lever et coucher du soleil
type SunTimes struct {
	Sunrise time.Time
	Sunset  time.Time
	Date    time.Time
}

// NewCalculator crée un nouveau calculateur
func NewCalculator(latitude, longitude float64, timezone string) *Calculator {
	return &Calculator{
		Latitude:  latitude,
		Longitude: longitude,
		Timezone:  timezone,
	}
}

// GetSunTimes calcule les heures du soleil pour une date donnée
func (c *Calculator) GetSunTimes(date time.Time) (*SunTimes, error) {
	// Charger le fuseau horaire
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return nil, err
	}

	// Normaliser la date au début de la journée dans le fuseau horaire local
	dateInTz := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)

	// CRITICAL FIX: La library suncalc attend TOUJOURS une date en UTC
	// Si on passe une date en timezone locale, elle calcule pour le mauvais jour
	// Convertir en UTC en préservant la date calendaire (pas l'instant)
	dateUTC := time.Date(date.Year(), date.Month(), date.Day(), 12, 0, 0, 0, time.UTC)

	// Calculer les heures du soleil
	times := suncalc.GetTimes(dateUTC, c.Latitude, c.Longitude)

	// GetTimes retourne une map[DayTimeName]DayTime où DayTime.Value est le time.Time
	sunrise := times[suncalc.Sunrise].Value
	sunset := times[suncalc.Sunset].Value

	return &SunTimes{
		Sunrise: sunrise.In(loc),
		Sunset:  sunset.In(loc),
		Date:    dateInTz,
	}, nil
}

// GetSunTimesToday retourne les heures du soleil pour aujourd'hui
func (c *Calculator) GetSunTimesToday() (*SunTimes, error) {
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		loc = time.UTC
	}
	return c.GetSunTimes(time.Now().In(loc))
}

// GetSunTimesTomorrow retourne les heures du soleil pour demain
func (c *Calculator) GetSunTimesTomorrow() (*SunTimes, error) {
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		loc = time.UTC
	}
	tomorrow := time.Now().In(loc).Add(24 * time.Hour)
	return c.GetSunTimes(tomorrow)
}

// GetSunTimesWithOffset retourne les heures du soleil avec des décalages
func (c *Calculator) GetSunTimesWithOffset(date time.Time, openOffsetMinutes, closeOffsetMinutes int) (*SunTimes, error) {
	times, err := c.GetSunTimes(date)
	if err != nil {
		return nil, err
	}

	times.Sunrise = times.Sunrise.Add(time.Duration(openOffsetMinutes) * time.Minute)
	times.Sunset = times.Sunset.Add(time.Duration(closeOffsetMinutes) * time.Minute)

	return times, nil
}
