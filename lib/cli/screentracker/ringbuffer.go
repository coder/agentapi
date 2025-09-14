package screentracker

// RingBuffer is a generic circular buffer that can store items of any type
type RingBuffer[T any] struct {
	items     []T
	nextIndex int
	count     int
	size      int
}

// NewRingBuffer creates a new ring buffer with the specified size
func NewRingBuffer[T any](size int) *RingBuffer[T] {
	return &RingBuffer[T]{
		items:     make([]T, size),
		nextIndex: 0,
		count:     0,
		size:      size,
	}
}

// Add adds an item to the ring buffer
func (r *RingBuffer[T]) Add(item T) {
	r.items[r.nextIndex] = item
	r.nextIndex = (r.nextIndex + 1) % len(r.items)
	if r.count < len(r.items) {
		r.count++
	}
}

// GetAll returns all items in the buffer, oldest first
func (b *RingBuffer[T]) GetAll() []T {
	result := make([]T, b.count)
	for i := 0; i < b.count; i++ {
		result[i] = b.items[(b.nextIndex-b.count+i+len(b.items))%len(b.items)]
	}
	return result
}

// Capacity returns the capacity of the ring buffer
func (b *RingBuffer[T]) Capacity() int {
	return b.size
}
