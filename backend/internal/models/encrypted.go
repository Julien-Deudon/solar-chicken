package models

import (
	"context"
	"fmt"
	"reflect"

	"github.com/julien-deudon/solar-chicken/backend/internal/secrets"
	"gorm.io/gorm/schema"
)

// encryptedSerializer chiffre un champ texte au repos (balise gorm:"serializer:encrypted").
type encryptedSerializer struct{}

func init() { schema.RegisterSerializer("encrypted", encryptedSerializer{}) }

func (encryptedSerializer) Scan(ctx context.Context, field *schema.Field, dst reflect.Value, dbValue interface{}) error {
	var raw string
	switch v := dbValue.(type) {
	case nil:
	case string:
		raw = v
	case []byte:
		raw = string(v)
	default:
		return fmt.Errorf("type %T inattendu pour un secret", dbValue)
	}
	plain, err := secrets.Decrypt(raw)
	if err != nil {
		return err
	}
	field.ReflectValueOf(ctx, dst).SetString(plain)
	return nil
}

func (encryptedSerializer) Value(ctx context.Context, field *schema.Field, dst reflect.Value, fieldValue interface{}) (interface{}, error) {
	s, _ := fieldValue.(string)
	return secrets.Encrypt(s)
}
