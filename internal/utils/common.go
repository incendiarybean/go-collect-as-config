package utils

import (
	"crypto/sha1"
	"encoding/base64"
	"math/rand/v2"
)

var UserAgents = []string{"Mozilla/5.0 (Windows NT 10.0; Win64; x64)", "AppleWebKit/537.36 (KHTML, like Gecko)", "Chrome/153.0.0.0", "Safari/537.36", "Edg/153.0.0.0"}

// Obtain a random user agent from the list of available entries
func RandomUserAgent() string {
	index := rand.IntN(len(UserAgents)-0) + 0

	return UserAgents[index]
}

// A function to generate a sha1 hash, for monitoring duplications within the data store
func Hash(str string) string {
	hasher := sha1.New()
	hasher.Write([]byte(str))
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}
