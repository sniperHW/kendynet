package kendynet

//go test -covermode=count -v -run=.
import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestBytebuffer(t *testing.T) {

	b := NewByteBuffer(64)
	assert.Equal(t, b.Cap(), uint64(128))
	assert.Equal(t, b.Len(), uint64(0))

	b.AppendByte(byte(1))
	assert.Equal(t, b.Len(), uint64(1))
	{
		r, err := b.GetByte(0)
		assert.Nil(t, err)
		assert.Equal(t, r, byte(1))
	}

	c := b.Clone()
	assert.Equal(t, b.Cap(), uint64(128))
	assert.Equal(t, b.Len(), uint64(1))

	b.PutByte(0, byte(2))

	{
		r, err := b.GetByte(0)
		assert.Nil(t, err)
		assert.Equal(t, r, byte(2))
	}

	{
		r, err := c.GetByte(0)
		assert.Nil(t, err)
		assert.Equal(t, r, byte(1))
	}

	b.PutByte(0, byte(1))
	b.AppendUint16(uint16(2))
	b.AppendInt16(int16(3))
	b.AppendUint32(uint32(4))
	b.AppendInt32(int32(5))
	b.AppendUint64(uint64(6))
	b.AppendInt64(int64(7))
	b.AppendString("hello")
	b.AppendBytes(([]byte)("world"))

	{
		reader := NewReader(b)
		{
			r, err := reader.GetByte()
			assert.Nil(t, err)
			assert.Equal(t, r, byte(1))
		}

		{
			r, err := reader.GetUint16()
			assert.Nil(t, err)
			assert.Equal(t, r, uint16(2))
		}

		{
			r, err := reader.GetInt16()
			assert.Nil(t, err)
			assert.Equal(t, r, int16(3))
		}

		{
			r, err := reader.GetUint32()
			assert.Nil(t, err)
			assert.Equal(t, r, uint32(4))
		}

		{
			r, err := reader.GetInt32()
			assert.Nil(t, err)
			assert.Equal(t, r, int32(5))
		}

		{
			r, err := reader.GetUint64()
			assert.Nil(t, err)
			assert.Equal(t, r, uint64(6))
		}

		{
			r, err := reader.GetInt64()
			assert.Nil(t, err)
			assert.Equal(t, r, int64(7))
		}

		{
			r, err := reader.GetString(5)
			assert.Nil(t, err)
			assert.Equal(t, r, "hello")
		}

		{
			r, err := reader.GetBytes(5)
			assert.Nil(t, err)
			assert.Equal(t, r, []byte("world"))
		}

	}

	{
		c := NewByteBuffer(b.Bytes(), b.Len())
		assert.Equal(t, c.Len(), b.Len())
		assert.Equal(t, c.Cap(), b.Len())
		assert.Equal(t, c.needcopy, true)

		c.PutByte(0, byte(2))
		assert.Equal(t, c.needcopy, false)

		{
			r, err := c.GetByte(0)
			assert.Nil(t, err)
			assert.Equal(t, r, byte(2))
		}

		{
			r, err := b.GetByte(0)
			assert.Nil(t, err)
			assert.Equal(t, r, byte(1))
		}

	}

	{
		s := strings.Repeat("a", 128)
		b := NewByteBuffer(s)
		assert.Equal(t, b.Len(), uint64(128))
		assert.Equal(t, b.Cap(), uint64(128))
		//expand
		b.AppendByte(byte(1))
		assert.Equal(t, b.Len(), uint64(129))
		assert.Equal(t, b.Cap(), uint64(256))
	}

	{
		s := strings.Repeat("a", 128)
		b := NewByteBuffer([]byte(s))
		assert.Equal(t, b.Len(), uint64(128))
		assert.Equal(t, b.Cap(), uint64(128))
		b.Reset()
		assert.Equal(t, b.Len(), uint64(0))
		assert.Equal(t, b.Cap(), uint64(128))
	}

}
