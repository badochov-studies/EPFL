package processor

import "unsafe"

func u64Toi64(num uint64) int64 {
	return *((*int64)(unsafe.Pointer(&num)))
}

func i64Tou64(num int64) uint64 {
	return *((*uint64)(unsafe.Pointer(&num)))
}
