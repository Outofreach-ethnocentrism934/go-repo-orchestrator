package workdir

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// ManagedRepoDirKey возвращает ключ managed-репозитория:
// <safeName>__<first8bytes(sha256(url))hex>.
func ManagedRepoDirKey(repoName, repoURL string) string {
	hash := sha256.Sum256([]byte(repoURL))
	hashSuffix := hex.EncodeToString(hash[:8])
	safeName := sanitizeDirPart(repoName)
	if safeName == "" {
		safeName = "repo"
	}

	return safeName + "__" + hashSuffix
}

func sanitizeDirPart(s string) string {
	b := strings.Builder{}
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '.' || ch == '-' || ch == '_' {
			b.WriteByte(ch)
			continue
		}
		b.WriteByte('_')
	}

	return strings.Trim(b.String(), "_")
}
