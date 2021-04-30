package aio

type sendqueue struct {
	head int
	tail int
	data []interface{}
	max  int
}

func newring(max int) sendqueue {
	r := sendqueue{
		max: max,
	}

	var l int
	if max == 0 {
		l = 100 + 1
	} else {
		l = max + 1
	}

	r.data = make([]interface{}, l, l)

	return r
}

func (r *sendqueue) setMax(max int) {
	if max > 0 {
		r.max = max
	}
}

func (r *sendqueue) grow() {
	data := make([]interface{}, len(r.data)*2-1, len(r.data)*2-1)
	i := 0
	for v := r.pop(); nil != v; v = r.pop() {
		data[i] = v
		i++
	}
	r.data = data
	r.head = 0
	r.tail = i
}

func (r *sendqueue) pop() interface{} {
	if r.head != r.tail {
		head := r.data[r.head]
		r.data[r.head] = nil
		r.head = (r.head + 1) % len(r.data)
		return head
	} else {
		return nil
	}
}

func (r *sendqueue) push(v interface{}) bool {
	if (r.tail+1)%len(r.data) != r.head {
		r.data[r.tail] = v
		r.tail = (r.tail + 1) % len(r.data)
		return true
	} else if r.max == 0 {
		r.grow()
		return r.push(v)
	} else {
		return false
	}
}

func (r *sendqueue) empty() bool {
	return r.head == r.tail
}
