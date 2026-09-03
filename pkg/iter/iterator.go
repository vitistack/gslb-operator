// provides helper functions for Iterators that allows chaining multiple iterators together
package iter

import (
	"iter"
	"slices"
)

type Iterable[T any] interface {
	Iter() Iterator[T]
}

type Iterator[T any] iter.Seq[T]

// creates an iterator from a slice
func FromSlice[T any](s []T) Iterator[T] {
	return func(yield func(T) bool) {
		for _, val := range s {
			if !yield(val) {
				return
			}
		}
	}
}

// creates an iterator from any iterable type that satisfies the iterable interface
func From[T any](it Iterable[T]) Iterator[T] {
	return it.Iter()
}

func FromSeq[T any](iter iter.Seq[T]) Iterator[T] {
	return Iterator[T](iter)
}

func (i Iterator[T]) Collect() []T {
	collection := make([]T, 0)
	for element := range i {
		collection = append(collection, element)
	}
	return collection
}

func (i Iterator[T]) Filter(filter func(T) bool) Iterator[T] {
	copy := i
	i = func(yield func(T) bool) {
		for element := range copy {
			if filter(element) {
				if !yield(element) {
					return
				}
			}
		}
	}
	return i
}

func Map[T, A any](iter Iterator[T], fn func(T) A) Iterator[A] {
	return func(yield func(A) bool) {
		for element := range iter {
			if !yield(fn(element)) {
				return
			}
		}
	}
}

func (i Iterator[T]) Map(fn func(T) T) Iterator[T] {
	copy := i
	i = func(yield func(T) bool) {
		for element := range copy {
			element = fn(element)
			if !yield(element) {
				return
			}
		}
	}
	return i
}

func (i Iterator[T]) Reverse() Iterator[T] {
	collect := i.Collect()
	slices.Reverse(collect)
	return FromSlice(collect)
}

func (i Iterator[T]) Take(n int) Iterator[T] {
	return func(yield func(T) bool) {
		counter := 0
		for element := range i {
			if counter >= n {
				return
			}

			if !yield(element) {
				return
			}

			counter++
		}
	}
}

func (i Iterator[T]) Skip(n int) Iterator[T] {
	return func(yield func(T) bool) {
		counter := 0
		for element := range i {
			if counter < n {
				counter++
				continue
			}

			if !yield(element) {
				return
			}

			counter++
		}
	}
}

func (i Iterator[T]) Tap(fn func(T)) Iterator[T] {
	return func(yield func(T) bool) {
		for element := range i {
			fn(element)

			if !yield(element) {
				return
			}
		}
	}
}

func (i Iterator[T]) Each(fn func(T)) {
	for element := range i {
		fn(element)
	}
}

func (i Iterator[T]) Find(pred func(T) bool) (T, bool) {
	for element := range i {
		if pred(element) {
			return element, true
		}
	}
	var zero T
	return zero, false
}

// this method will be allowed once go 1.27 releases!!
//func (i Iterator[T]) Reduce[A any](initial A, fn func(A, T) A) A {
//	acc := initial
//	for element := range i {
//		acc = fn(acc, element)
//	}
//	return acc
//}

func (i Iterator[T]) Any(pred func(T) bool) bool {
	for element := range i {
		if pred(element) {
			return true
		}
	}
	return false
}

func (i Iterator[T]) All(pred func(T) bool) bool {
	for element := range i {
		if !pred(element) {
			return false
		}
	}
	return true
}

func (i Iterator[T]) Count() int {
	counter := 0
	for range i {
		counter++
	}
	return counter
}

func (i Iterator[T]) Contains(cmp func(T) bool) bool {
	for element := range i {
		if cmp(element) {
			return true
		}
	}
	return false
}
