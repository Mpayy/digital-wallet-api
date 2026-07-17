package hashutil

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func HashPayload(payload any) (string, error) {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	hashBytes := sha256.Sum256(bytes)
	hashString := hex.EncodeToString(hashBytes[:])

	return hashString, nil
}
