// SPDX-License-Identifier: Apache-2.0

package slices

func Any[T any](list []T, predicate func(T) bool) bool {
	for _, e := range list {
		if predicate(e) {
			return true
		}
	}
	return false
}

func Filter[T any](list []T, predicate func(T) bool) []T {
	var result []T
	for _, e := range list {
		if predicate(e) {
			result = append(result, e)
		}
	}
	return result
}

func Contains[T comparable](list []T, elem T) bool {
	return Any(list, func(e T) bool { return e == elem })
}

func AppendUnique[T comparable](list []T, elem T) []T {
	for _, e := range list {
		if e == elem {
			return list
		}
	}
	return append(list, elem)
}

func Remove[T comparable](list []T, elem T) []T {
	for i, e := range list {
		if e == elem {
			return RemoveAt(list, i)
		}
	}
	return list
}

func RemoveAt[T any](list []T, index int) []T {
	list[index] = list[len(list)-1]
	return list[:len(list)-1]
}
