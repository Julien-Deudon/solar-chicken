package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Anchor est le repère d'une ouverture ou d'une fermeture.
type Anchor string

const (
	AnchorSunrise     Anchor = "sunrise"      // lever du soleil
	AnchorSunset      Anchor = "sunset"       // coucher du soleil
	AnchorFixed       Anchor = "fixed"        // heure fixe
	AnchorDeviceOpen  Anchor = "device_open"  // ouverture d'un autre appareil
	AnchorDeviceClose Anchor = "device_close" // fermeture d'un autre appareil
)

func (a Anchor) Valid() bool {
	switch a {
	case AnchorSunrise, AnchorSunset, AnchorFixed, AnchorDeviceOpen, AnchorDeviceClose:
		return true
	}
	return false
}

// Rule décrit quand une porte s'ouvre et se ferme.
// Chaque moment = repère + décalage en minutes, éventuellement borné (pas avant / pas après).
type Rule struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	DeviceID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"deviceId"`

	OpenAnchor        Anchor     `gorm:"type:varchar(20);not null" json:"openAnchor"`
	OpenOffsetMinutes int        `gorm:"not null" json:"openOffsetMinutes"`
	OpenFixedTime     *TimeOfDay `gorm:"type:time" json:"openFixedTime"`
	OpenRefDeviceID   *uuid.UUID `gorm:"type:uuid" json:"openRefDeviceId"`
	OpenNotBefore     *TimeOfDay `gorm:"type:time" json:"openNotBefore"`
	OpenNotAfter      *TimeOfDay `gorm:"type:time" json:"openNotAfter"`

	CloseAnchor        Anchor     `gorm:"type:varchar(20);not null" json:"closeAnchor"`
	CloseOffsetMinutes int        `gorm:"not null" json:"closeOffsetMinutes"`
	CloseFixedTime     *TimeOfDay `gorm:"type:time" json:"closeFixedTime"`
	CloseRefDeviceID   *uuid.UUID `gorm:"type:uuid" json:"closeRefDeviceId"`
	CloseNotBefore     *TimeOfDay `gorm:"type:time" json:"closeNotBefore"`
	CloseNotAfter      *TimeOfDay `gorm:"type:time" json:"closeNotAfter"`

	// Lumière (seulement si l'appareil en possède une)
	LightBeforeOpenMinutes  int  `gorm:"not null" json:"lightBeforeOpenMinutes"`
	LightBeforeCloseMinutes int  `gorm:"not null" json:"lightBeforeCloseMinutes"`
	EnableLightMorning      bool `gorm:"not null" json:"enableLightMorning"`
	EnableLightEvening      bool `gorm:"not null" json:"enableLightEvening"`
	LightOffDelayMinutes    int  `gorm:"not null" json:"lightOffDelayMinutes"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (r *Rule) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

// Moment regroupe les champs d'une ouverture ou d'une fermeture.
type Moment struct {
	Anchor        Anchor
	OffsetMinutes int
	FixedTime     *TimeOfDay
	RefDeviceID   *uuid.UUID
	NotBefore     *TimeOfDay
	NotAfter      *TimeOfDay
}

func (r *Rule) Open() Moment {
	return Moment{r.OpenAnchor, r.OpenOffsetMinutes, r.OpenFixedTime, r.OpenRefDeviceID, r.OpenNotBefore, r.OpenNotAfter}
}

func (r *Rule) Close() Moment {
	return Moment{r.CloseAnchor, r.CloseOffsetMinutes, r.CloseFixedTime, r.CloseRefDeviceID, r.CloseNotBefore, r.CloseNotAfter}
}

func pick(lang, fr, en string) string {
	if lang == "en" {
		return en
	}
	return fr
}

func (m Moment) validate(lang, label string, self uuid.UUID) error {
	if !m.Anchor.Valid() {
		return fmt.Errorf(pick(lang, "%s : repère %q inconnu", "%s: unknown anchor %q"), label, m.Anchor)
	}
	if m.OffsetMinutes < -720 || m.OffsetMinutes > 720 {
		return fmt.Errorf(pick(lang, "%s : décalage hors limites (±720 min)", "%s: offset out of range (±720 min)"), label)
	}
	if m.Anchor == AnchorFixed && m.FixedTime == nil {
		return fmt.Errorf(pick(lang, "%s : heure fixe manquante", "%s: fixed time missing"), label)
	}
	if (m.Anchor == AnchorDeviceOpen || m.Anchor == AnchorDeviceClose) && (m.RefDeviceID == nil || *m.RefDeviceID == uuid.Nil) {
		return fmt.Errorf(pick(lang, "%s : appareil de référence manquant", "%s: reference device missing"), label)
	}
	if m.RefDeviceID != nil && *m.RefDeviceID == self {
		return fmt.Errorf(pick(lang, "%s : un appareil ne peut pas se référencer lui-même", "%s: a device cannot reference itself"), label)
	}
	if m.NotBefore != nil && m.NotAfter != nil &&
		m.NotBefore.Hour*60+m.NotBefore.Minute > m.NotAfter.Hour*60+m.NotAfter.Minute {
		return fmt.Errorf(pick(lang, "%s : « jamais avant » est après « jamais après »", "%s: “not before” is after “not after”"), label)
	}
	return nil
}

// Validate vérifie la cohérence de la règle (messages en français).
func (r *Rule) Validate() error { return r.ValidateLang("fr") }

// ValidateLang vérifie la cohérence de la règle, messages en "fr" ou "en".
func (r *Rule) ValidateLang(lang string) error {
	if err := r.Open().validate(lang, pick(lang, "Ouverture", "Opening"), r.DeviceID); err != nil {
		return err
	}
	if err := r.Close().validate(lang, pick(lang, "Fermeture", "Closing"), r.DeviceID); err != nil {
		return err
	}
	for _, v := range []int{r.LightBeforeOpenMinutes, r.LightBeforeCloseMinutes, r.LightOffDelayMinutes} {
		if v < 0 || v > 120 {
			return fmt.Errorf(pick(lang, "lumière : durée hors limites (0 à 120 min)", "light: duration out of range (0 to 120 min)"))
		}
	}
	return nil
}
