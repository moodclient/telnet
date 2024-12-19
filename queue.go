package telnet

type queue[T any] struct {
	buffer     []T
	maxSize    int
	startIndex int
	endIndex   int
}

func newQueue[T any](size int) *queue[T] {
	return &queue[T]{
		buffer:  make([]T, size),
		maxSize: size,
	}
}

func (q *queue[T]) straighten() {
	if q.startIndex == 0 {
		return
	}

	len := q.endIndex - q.startIndex

	if len > 0 {
		copy(q.buffer[:len], q.buffer[q.startIndex:q.endIndex])
	}

	q.startIndex = 0
	q.endIndex = len
}

func (q *queue[T]) Queue(elements ...T) {
	for i := 0; i < len(elements); i++ {
		if q.endIndex < len(q.buffer) {
			q.buffer[q.endIndex] = elements[i]
			q.endIndex++
			continue
		}

		q.straighten()

		if q.endIndex*100/q.maxSize > 80 {
			newMaxSize := q.maxSize * 2
			newBuffer := make([]T, newMaxSize)
			copy(newBuffer, q.buffer)
			q.buffer = newBuffer
			q.maxSize = newMaxSize
		}

		i--
	}
}

func (q *queue[T]) Dequeue() T {
	if q.startIndex == q.endIndex {
		var zero T
		return zero
	}

	value := q.buffer[q.startIndex]
	q.startIndex++
	return value
}

func (q *queue[T]) DropElements(n int) {
	newStart := q.startIndex + n
	if newStart > q.endIndex {
		q.startIndex = q.endIndex
	} else {
		q.startIndex = newStart
	}
}

func (q *queue[T]) Buffer() []T {
	return q.buffer[q.startIndex:q.endIndex]
}

func (q *queue[T]) Len() int {
	return q.endIndex - q.startIndex
}
