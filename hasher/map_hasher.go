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
	sortedKeys := sortMapKeys(m)
	strToHash := buildString(m, sortedKeys)

	if strToHash == "" {
		return ""
	}

	hash := sha256.Sum256([]byte(strToHash))
	return hex.EncodeToString(hash[:])
}

func sortMapKeys(m map[string]string) []string {
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
		str = fmt.Sprintf("%s%s:%s;", str, key, m[key])
	}
	return str
}
