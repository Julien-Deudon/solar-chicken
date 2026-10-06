package secrets

import "testing"

func TestRoundTripAndPlaintextCompatibility(t *testing.T) {
	if err := Init(""); err != nil {
		t.Fatal(err)
	}
	if v, _ := Encrypt("abc"); v != "abc" {
		t.Fatalf("sans clé, la valeur doit rester en clair : %q", v)
	}
	if err := Init("une phrase secrète quelconque"); err != nil {
		t.Fatal(err)
	}
	enc, err := Encrypt("cle-omlet-123")
	if err != nil || !IsEncrypted(enc) || enc == "cle-omlet-123" {
		t.Fatalf("chiffrement : %q %v", enc, err)
	}
	again, _ := Encrypt("cle-omlet-123")
	if again == enc {
		t.Fatalf("deux chiffrements identiques : le nonce doit être aléatoire")
	}
	if dec, err := Decrypt(enc); err != nil || dec != "cle-omlet-123" {
		t.Fatalf("déchiffrement : %q %v", dec, err)
	}
	if dec, _ := Decrypt("ancienne-valeur-en-clair"); dec != "ancienne-valeur-en-clair" {
		t.Fatalf("une valeur en clair doit rester lisible")
	}
	if double, _ := Encrypt(enc); double != enc {
		t.Fatalf("une valeur déjà chiffrée ne doit pas être rechiffrée")
	}
	_ = Init("une autre clé")
	if _, err := Decrypt(enc); err == nil {
		t.Fatalf("une autre clé ne doit pas déchiffrer")
	}
	_ = Init("")
	if _, err := Decrypt(enc); err != ErrNoKey {
		t.Fatalf("sans clé : %v", err)
	}
}
