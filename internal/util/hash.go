package util

import (
	"hash/fnv"
)

func HashStringToUInt64(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}
