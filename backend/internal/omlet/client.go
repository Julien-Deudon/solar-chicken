package omlet

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const DefaultBaseURL = "https://x107.omlet.co.uk/api/v1"

// Client est le client de l'API Omlet (https://smart.omlet.com/developers/api).
type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
	// Délais entre relances des lectures et écritures de configuration (pas des actions).
	RetryDelays []time.Duration
}

func NewClient(apiKey string) *Client {
	return &Client{
		BaseURL:     DefaultBaseURL,
		APIKey:      apiKey,
		HTTP:        &http.Client{Timeout: 20 * time.Second},
		RetryDelays: []time.Duration{2 * time.Second, 6 * time.Second},
	}
}

// APIError est une réponse HTTP en erreur.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string { return fmt.Sprintf("API Omlet HTTP %d : %s", e.Status, e.Message) }

func retryable(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Status == http.StatusTooManyRequests || apiErr.Status >= 500
	}
	return err != nil // erreur réseau / délai dépassé
}

func (c *Client) do(ctx context.Context, method, path string, body, out interface{}, retry bool) error {
	attempts := 1
	if retry {
		attempts += len(c.RetryDelays)
	}
	var err error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.RetryDelays[i-1]):
			}
		}
		err = c.once(ctx, method, path, body, out)
		if err == nil || !retryable(err) {
			return err
		}
	}
	return err
}

func (c *Client) once(ctx context.Context, method, path string, body, out interface{}) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("API Omlet injoignable : %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		var e ErrorResponse
		msg := string(data)
		if json.Unmarshal(data, &e) == nil && e.Message != "" {
			msg = e.Message
		}
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return &APIError{Status: resp.StatusCode, Message: msg}
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("réponse Omlet illisible : %w", err)
		}
	}
	return nil
}

// ListDevices retourne tous les appareils du compte.
func (c *Client) ListDevices(ctx context.Context) ([]Device, error) {
	var devices []Device
	err := c.do(ctx, http.MethodGet, "/device", nil, &devices, true)
	return devices, err
}

// GetDevice retourne l'état d'un appareil.
func (c *Client) GetDevice(ctx context.Context, deviceID string) (*Device, error) {
	var d Device
	if err := c.do(ctx, http.MethodGet, "/device/"+deviceID, nil, &d, true); err != nil {
		return nil, err
	}
	return &d, nil
}

// Action envoie une action (open, close, stop, on, off). Jamais relancée ici :
// c'est le moteur qui vérifie l'état de la porte avant toute nouvelle tentative.
func (c *Client) Action(ctx context.Context, deviceID, action string) error {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/device/%s/action/%s", deviceID, action), nil, nil, false)
}

// SetDoorTimes passe la porte en mode horaire avec les heures données (HH:MM).
// La section "door" est relue puis renvoyée en entier pour ne perdre aucun autre réglage.
func (c *Client) SetDoorTimes(ctx context.Context, deviceID, openTime, closeTime string) error {
	var cfg map[string]json.RawMessage
	if err := c.do(ctx, http.MethodGet, "/device/"+deviceID+"/configuration", nil, &cfg, true); err != nil {
		return err
	}
	door := map[string]interface{}{}
	if raw, ok := cfg["door"]; ok {
		if err := json.Unmarshal(raw, &door); err != nil {
			return fmt.Errorf("configuration porte illisible : %w", err)
		}
	}
	if len(door) == 0 {
		return errors.New("configuration porte absente : appareil non compatible")
	}
	door["openMode"], door["closeMode"] = "time", "time"
	door["openTime"], door["closeTime"] = openTime, closeTime
	return c.do(ctx, http.MethodPatch, "/device/"+deviceID+"/configuration", map[string]interface{}{"door": door}, nil, true)
}
