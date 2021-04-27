package util

//go test -covermode=count -v -run=TestFlag
import (
	"github.com/stretchr/testify/assert"
	"testing"
)

const (
	fclosed  = uint32(1 << 1)
	frclosed = uint32(1 << 2)
	fwclosed = uint32(1 << 3)
)

func TestFlag(t *testing.T) {
	var f Flag
	f.Set(fclosed)
	assert.Equal(t, true, f.Test(fclosed))
	assert.Equal(t, false, f.Test(frclosed))
	assert.Equal(t, true, f.Test(fclosed|frclosed))
	f.Set(fwclosed)
	assert.Equal(t, true, f.Test(fwclosed))
	f.Clear(fclosed)
	assert.Equal(t, false, f.Test(fclosed))
	assert.Equal(t, true, f.Test(fwclosed))
	f.Set(fclosed | frclosed | fclosed)
	assert.Equal(t, true, f.Test(fclosed))
	assert.Equal(t, true, f.Test(fwclosed))
	assert.Equal(t, true, f.Test(frclosed))

	f.Clear(fclosed | fwclosed)
	assert.Equal(t, true, f.Test(frclosed))
	assert.Equal(t, false, f.Test(fclosed))
	assert.Equal(t, false, f.Test(fwclosed))

}
