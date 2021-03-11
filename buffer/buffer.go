package buffer

import (
	"encoding/binary"
)

type Buffer struct {
	bs   []byte
	pool *Pool
}

func New(o []byte) *Buffer {
	return &Buffer{
		bs: o,
	}
}

//implement io.Writer
func (b *Buffer) Write(bytes []byte) (int, error) {
	b.AppendBytes(bytes)
	return len(bytes), nil
}

func (b *Buffer) AppendByte(v byte) {
	b.bs = append(b.bs, v)
}

func (b *Buffer) AppendString(s string) {
	b.bs = append(b.bs, s...)
}

func (b *Buffer) AppendBytes(bytes []byte) {
	b.bs = append(b.bs, bytes...)
}

func (b *Buffer) AppendUint16(u16 uint16) {
	bu := make([]byte, 2)
	binary.BigEndian.PutUint16(bu, u16)
	b.AppendBytes(bu)
}

func (b *Buffer) AppendUint32(u32 uint32) {
	bu := make([]byte, 4)
	binary.BigEndian.PutUint32(bu, u32)
	b.AppendBytes(bu)
}

func (b *Buffer) AppendUint64(u64 uint64) {
	bu := make([]byte, 8)
	binary.BigEndian.PutUint64(bu, u64)
	b.AppendBytes(bu)
}

func (b *Buffer) AppendInt16(i16 int16) {
	b.AppendUint16(uint16(i16))
}

func (b *Buffer) AppendInt32(i32 int32) {
	b.AppendUint32(uint32(i32))
}

func (b *Buffer) AppendInt64(i64 int64) {
	b.AppendUint64(uint64(i64))
}

func (b *Buffer) Bytes() []byte {
	return b.bs
}

func (b *Buffer) Len() int {
	return len(b.bs)
}

func (b *Buffer) Cap() int {
	return cap(b.bs)
}

func (b *Buffer) Reset() {
	b.bs = b.bs[:0]
}

func (b *Buffer) Free() {
	if nil != b.pool {
		b.pool.put(b)
	}
}

type BufferReader struct {
	buffer *Buffer
	offset int
}

func NewReader(buffer *Buffer) *BufferReader {
	return &BufferReader{buffer: buffer}
}

func (this *BufferReader) GetByte() byte {
	if this.offset+1 >= len(this.buffer.bs) {
		return 0
	} else {
		ret := this.buffer.bs[this.offset]
		this.offset += 1
		return ret
	}
}

func (this *BufferReader) GetUint16() uint16 {
	if this.offset+2 >= len(this.buffer.bs) {
		return 0
	} else {
		ret := binary.BigEndian.Uint16(this.buffer.bs[this.offset : this.offset+2])
		this.offset += 2
		return ret
	}
}

func (this *BufferReader) GetInt16() int16 {
	return int16(this.GetUint16())
}

func (this *BufferReader) GetUint32() uint32 {
	if this.offset+4 >= len(this.buffer.bs) {
		return 0
	} else {
		ret := binary.BigEndian.Uint32(this.buffer.bs[this.offset : this.offset+4])
		this.offset += 4
		return ret
	}
}

func (this *BufferReader) GetInt32() int32 {
	return int32(this.GetUint32())
}

func (this *BufferReader) GetUint64() uint64 {
	if this.offset+8 >= len(this.buffer.bs) {
		return 0
	} else {
		ret := binary.BigEndian.Uint64(this.buffer.bs[this.offset : this.offset+8])
		this.offset += 8
		return ret
	}
}

func (this *BufferReader) GetInt64() int64 {
	return int64(this.GetUint64())
}

func (this *BufferReader) GetString(size int) string {
	return string(this.GetBytes(size))
}

func (this *BufferReader) GetBytes(size int) []byte {
	if len(this.buffer.bs)-this.offset < size {
		size = len(this.buffer.bs) - this.offset
	}
	ret := this.buffer.bs[this.offset : this.offset+size]
	this.offset += size
	return ret
}
