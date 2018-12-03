package hasher

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
)

type MapHasher struct {
}

func (h *MapHasher) Hash(m map[string]string) string {
	sortedKeys := sortedMapKeys(m)
	strToHash := buildString(m, sortedKeys)

	if strToHash == "" {
		return ""
	}

	hash := sha256.Sum256([]byte(strToHash))
	return hex.EncodeToString(hash[:])
}

func sortedMapKeys(m map[string]string) []string {
	keys := []string{}
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func buildString(m map[string]string, sortedKeys []string) string {
	var str string
	for _, key := range sortedKeys {
		keyHash := sha256.Sum256([]byte(key))
		valueHash := sha256.Sum256([]byte(m[key]))
		str = fmt.Sprintf("%s%s%s", str, keyHash, valueHash)
	}
	return str
}
