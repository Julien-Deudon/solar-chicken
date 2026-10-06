package suncalc

import (
	"testing"
	"time"
)

func TestGetSunTimes_CorrectDate(t *testing.T) {
	// Test que le SunCalculator calcule bien pour le jour demandé, pas la veille
	calc := NewCalculator(48.8566, 2.3522, "Europe/Paris") // Paris coordinates

	// Test pour le 20 décembre 2025
	loc, _ := time.LoadLocation("Europe/Paris")
	testDate := time.Date(2025, 12, 20, 15, 0, 0, 0, loc) // 20 déc 2025, 15h Paris

	sunTimes, err := calc.GetSunTimes(testDate)
	if err != nil {
		t.Fatalf("GetSunTimes failed: %v", err)
	}

	// Vérifier que la date du lever/coucher est bien le 20 décembre
	sunriseDate := sunTimes.Sunrise.Format("2006-01-02")
	sunsetDate := sunTimes.Sunset.Format("2006-01-02")
	expectedDate := "2025-12-20"

	if sunriseDate != expectedDate {
		t.Errorf("Sunrise date incorrect: got %s, want %s", sunriseDate, expectedDate)
	}

	if sunsetDate != expectedDate {
		t.Errorf("Sunset date incorrect: got %s, want %s", sunsetDate, expectedDate)
	}

	// Vérifier que les heures sont raisonnables pour Paris en décembre
	// En hiver, lever vers 8h30, coucher vers 17h
	sunriseHour := sunTimes.Sunrise.Hour()
	sunsetHour := sunTimes.Sunset.Hour()

	if sunriseHour < 7 || sunriseHour > 9 {
		t.Errorf("Sunrise hour seems wrong: %d (expected between 7-9)", sunriseHour)
	}

	if sunsetHour < 16 || sunsetHour > 18 {
		t.Errorf("Sunset hour seems wrong: %d (expected between 16-18)", sunsetHour)
	}
}

func TestGetSunTimes_EveningTime(t *testing.T) {
	// Test critique: vérifier qu'on calcule le bon jour même si on demande en soirée
	calc := NewCalculator(48.8566, 2.3522, "Europe/Paris")

	loc, _ := time.LoadLocation("Europe/Paris")
	// 20 décembre 2025, 20h (après le coucher du soleil)
	eveningTime := time.Date(2025, 12, 20, 20, 0, 0, 0, loc)

	sunTimes, err := calc.GetSunTimes(eveningTime)
	if err != nil {
		t.Fatalf("GetSunTimes failed: %v", err)
	}

	// Doit TOUJOURS calculer pour le 20, même si on demande à 20h
	sunriseDate := sunTimes.Sunrise.Format("2006-01-02")
	sunsetDate := sunTimes.Sunset.Format("2006-01-02")
	expectedDate := "2025-12-20"

	if sunriseDate != expectedDate {
		t.Errorf("Sunrise date incorrect when called in evening: got %s, want %s", sunriseDate, expectedDate)
	}

	if sunsetDate != expectedDate {
		t.Errorf("Sunset date incorrect when called in evening: got %s, want %s", sunsetDate, expectedDate)
	}
}

func TestGetSunTimesWithOffset(t *testing.T) {
	calc := NewCalculator(48.8566, 2.3522, "Europe/Paris")

	loc, _ := time.LoadLocation("Europe/Paris")
	testDate := time.Date(2025, 12, 20, 12, 0, 0, 0, loc)

	// Test avec offset: -30 min pour ouverture, +30 min pour fermeture
	sunTimes, err := calc.GetSunTimesWithOffset(testDate, -30, 30)
	if err != nil {
		t.Fatalf("GetSunTimesWithOffset failed: %v", err)
	}

	// Calculer sans offset pour comparer
	sunTimesNoOffset, _ := calc.GetSunTimes(testDate)

	// L'ouverture doit être 30 min AVANT le lever du soleil
	expectedSunrise := sunTimesNoOffset.Sunrise.Add(-30 * time.Minute)
	if !sunTimes.Sunrise.Equal(expectedSunrise) {
		t.Errorf("Sunrise offset incorrect: got %s, want %s",
			sunTimes.Sunrise.Format("15:04:05"),
			expectedSunrise.Format("15:04:05"))
	}

	// La fermeture doit être 30 min APRÈS le coucher du soleil
	expectedSunset := sunTimesNoOffset.Sunset.Add(30 * time.Minute)
	if !sunTimes.Sunset.Equal(expectedSunset) {
		t.Errorf("Sunset offset incorrect: got %s, want %s",
			sunTimes.Sunset.Format("15:04:05"),
			expectedSunset.Format("15:04:05"))
	}
}

func TestGetSunTimes_DifferentTimezones(t *testing.T) {
	testCases := []struct {
		name     string
		lat      float64
		lon      float64
		timezone string
		date     string
	}{
		{"Paris", 48.8566, 2.3522, "Europe/Paris", "2025-12-20"},
		{"New York", 40.7128, -74.0060, "America/New_York", "2025-12-20"},
		{"Tokyo", 35.6762, 139.6503, "Asia/Tokyo", "2025-12-20"},
		{"Sydney", -33.8688, 151.2093, "Australia/Sydney", "2025-12-20"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			calc := NewCalculator(tc.lat, tc.lon, tc.timezone)

			loc, err := time.LoadLocation(tc.timezone)
			if err != nil {
				t.Fatalf("Invalid timezone %s: %v", tc.timezone, err)
			}

			testDate := time.Date(2025, 12, 20, 12, 0, 0, 0, loc)

			sunTimes, err := calc.GetSunTimes(testDate)
			if err != nil {
				t.Fatalf("GetSunTimes failed for %s: %v", tc.name, err)
			}

			// Vérifier que la date est correcte
			sunriseDate := sunTimes.Sunrise.Format("2006-01-02")
			sunsetDate := sunTimes.Sunset.Format("2006-01-02")

			if sunriseDate != tc.date {
				t.Errorf("%s: Sunrise date incorrect: got %s, want %s", tc.name, sunriseDate, tc.date)
			}

			if sunsetDate != tc.date {
				t.Errorf("%s: Sunset date incorrect: got %s, want %s", tc.name, sunsetDate, tc.date)
			}

			// Vérifier que sunrise est avant sunset
			if sunTimes.Sunrise.After(sunTimes.Sunset) {
				t.Errorf("%s: Sunrise (%s) is after sunset (%s)",
					tc.name,
					sunTimes.Sunrise.Format("15:04:05"),
					sunTimes.Sunset.Format("15:04:05"))
			}
		})
	}
}

func TestGetSunTimesToday(t *testing.T) {
	calc := NewCalculator(48.8566, 2.3522, "Europe/Paris")

	sunTimes, err := calc.GetSunTimesToday()
	if err != nil {
		t.Fatalf("GetSunTimesToday failed: %v", err)
	}

	// Vérifier que c'est bien aujourd'hui
	loc, _ := time.LoadLocation("Europe/Paris")
	today := time.Now().In(loc).Format("2006-01-02")

	sunriseDate := sunTimes.Sunrise.Format("2006-01-02")
	sunsetDate := sunTimes.Sunset.Format("2006-01-02")

	if sunriseDate != today {
		t.Errorf("GetSunTimesToday sunrise not today: got %s, want %s", sunriseDate, today)
	}

	if sunsetDate != today {
		t.Errorf("GetSunTimesToday sunset not today: got %s, want %s", sunsetDate, today)
	}
}

func TestGetSunTimesTomorrow(t *testing.T) {
	calc := NewCalculator(48.8566, 2.3522, "Europe/Paris")

	sunTimes, err := calc.GetSunTimesTomorrow()
	if err != nil {
		t.Fatalf("GetSunTimesTomorrow failed: %v", err)
	}

	// Vérifier que c'est bien demain
	loc, _ := time.LoadLocation("Europe/Paris")
	tomorrow := time.Now().In(loc).Add(24 * time.Hour).Format("2006-01-02")

	sunriseDate := sunTimes.Sunrise.Format("2006-01-02")
	sunsetDate := sunTimes.Sunset.Format("2006-01-02")

	if sunriseDate != tomorrow {
		t.Errorf("GetSunTimesTomorrow sunrise not tomorrow: got %s, want %s", sunriseDate, tomorrow)
	}

	if sunsetDate != tomorrow {
		t.Errorf("GetSunTimesTomorrow sunset not tomorrow: got %s, want %s", sunsetDate, tomorrow)
	}
}
