package iter

import "iter"

type Iterator2[K, V any] iter.Seq2[K, V]

func FromMap[K comparable, V any](mp map[K]V) Iterator2[K, V] {
	return func(yield func(K, V) bool) {
		for key, val := range mp {
			if !yield(key, val) {
				return
			}
		}
	}
}

func FromSeq2[K any, V any](i iter.Seq2[K, V]) Iterator2[K, V] {
	return Iterator2[K, V](i)
}

func (i Iterator2[K, V]) Filter(pred func(K, V) bool) Iterator2[K, V] {
	copy := i
	i = func(yield func(K, V) bool) {
		for key, val := range copy {
			if pred(key, val) {
				if !yield(key, val) {
					return
				}
			}
		}
	}

	return i
}

func (i Iterator2[K, V]) Take(n int) Iterator2[K, V] {
	return func(yield func(K, V) bool) {
		counter := 0

		for key, val := range i {
			if counter >= n {
				return
			}

			if !yield(key, val) {
				return
			}

			counter++
		}
	}
}

func (i Iterator2[K, V]) Skip(n int) Iterator2[K, V] {
	return func(yield func(K, V) bool) {
		counter := 0
		for key, val := range i {
			if counter <= n {
				continue
			}

			if !yield(key, val) {
				return
			}

			counter++
		}
	}
}

func (i Iterator2[K, V]) Any(pred func(K, V) bool) bool {
	for key, val := range i {
		if pred(key, val) {
			return true
		}
	}
	return false
}

func (i Iterator2[K, V]) All(pred func(K, V) bool) bool {
	for key, val := range i {
		if !pred(key, val) {
			return false
		}
	}
	return true
}

func (i Iterator2[K, V]) Count() int {
	counter := 0
	for range i {
		counter++
	}
	return counter
}

func (i Iterator2[K, V]) Find(pred func(K, V) bool) (K, V, bool) {
	for key, val := range i {
		if pred(key, val) {
			return key, val, true
		}
	}

	var zeroKey K
	var zeroVal V
	return zeroKey, zeroVal, false
}

func (i Iterator2[K, V]) Each(fn func(K, V)) {
	for key, val := range i {
		fn(key, val)
	}
}

// this sucks but at least it will work, just no chaining
func Collect[K comparable, V any](i Iterator2[K, V]) map[K]V {
	mp := make(map[K]V)
	for key, val := range i {
		mp[key] = val
	}

	return mp
}
