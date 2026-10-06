// Package secrets chiffre au repos les secrets des utilisateurs (clé API Omlet, jetons Telegram).
//
// AES-256-GCM, clé dérivée de SECRET_KEY. Une valeur chiffrée commence par "enc:v1:".
// Une valeur sans ce préfixe est lue telle quelle (données d'avant le chiffrement)
// et sera chiffrée au prochain enregistrement.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
)

const prefix = "enc:v1:"

var (
	mu  sync.RWMutex
	gcm cipher.AEAD
)

// ErrNoKey : une valeur chiffrée est lue alors qu'aucune clé n'est configurée.
var ErrNoKey = errors.New("secret chiffré mais SECRET_KEY absente : impossible de le relire")

// Init configure la clé. Une chaîne hexadécimale ou base64 de 32 octets est utilisée telle quelle,
// toute autre chaîne est dérivée par SHA-256. Une chaîne vide désactive le chiffrement.
func Init(secret string) error {
	mu.Lock()
	defer mu.Unlock()
	secret = strings.TrimSpace(secret)
	if secret == "" {
		gcm = nil
		return nil
	}
	key := decodeKey(secret)
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	gcm = g
	return nil
}

func decodeKey(s string) []byte {
	if b, err := hex.DecodeString(s); err == nil && len(b) == 32 {
		return b
	}
	if b, err := base64.StdEncoding.DecodeString(s); err == nil && len(b) == 32 {
		return b
	}
	sum := sha256.Sum256([]byte(s))
	return sum[:]
}

// Enabled indique si une clé est configurée.
func Enabled() bool {
	mu.RLock()
	defer mu.RUnlock()
	return gcm != nil
}

// IsEncrypted indique si la valeur est déjà chiffrée.
func IsEncrypted(s string) bool { return strings.HasPrefix(s, prefix) }

// Encrypt chiffre une valeur (inchangée si vide, déjà chiffrée ou si aucune clé n'est configurée).
func Encrypt(plain string) (string, error) {
	mu.RLock()
	g := gcm
	mu.RUnlock()
	if plain == "" || g == nil || IsEncrypted(plain) {
		return plain, nil
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := g.Seal(nonce, nonce, []byte(plain), nil)
	return prefix + base64.RawStdEncoding.EncodeToString(sealed), nil
}

// Decrypt relit une valeur (une valeur non chiffrée est rendue telle quelle).
func Decrypt(s string) (string, error) {
	if !IsEncrypted(s) {
		return s, nil
	}
	mu.RLock()
	g := gcm
	mu.RUnlock()
	if g == nil {
		return "", ErrNoKey
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(s, prefix))
	if err != nil || len(raw) < g.NonceSize() {
		return "", errors.New("secret chiffré illisible")
	}
	plain, err := g.Open(nil, raw[:g.NonceSize()], raw[g.NonceSize():], nil)
	if err != nil {
		return "", errors.New("secret chiffré avec une autre clé (SECRET_KEY a changé ?)")
	}
	return string(plain), nil
}
