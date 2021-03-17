package pb

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	//"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/buffer"
	"reflect"
)

const (
	PBHeaderSize int = 4
	pbIdSize     int = 4
)

var (
	nameToTypeID = map[string]uint32{}
	idToMeta     = map[uint32]reflect.Type{}
)

func newMessage(id uint32) (msg proto.Message, err error) {
	if mt, ok := idToMeta[id]; ok {
		msg = reflect.New(mt).Interface().(proto.Message)
	} else {
		err = fmt.Errorf("not found %d", id)
	}
	return
}

//根据名字注册实例
func Register(msg proto.Message, id uint32) (err error) {
	if _, ok := idToMeta[id]; ok {
		err = fmt.Errorf("duplicate id:%d", id)
		return
	}

	tt := reflect.TypeOf(msg)
	name := tt.String()

	if _, ok := nameToTypeID[name]; ok {
		err = fmt.Errorf("%s already register", name)
		return
	}

	nameToTypeID[name] = id
	idToMeta[id] = tt.Elem()
	return nil
}

func Encode(o interface{}, b *buffer.Buffer, maxMsgSize int) (r *buffer.Buffer, e error) {
	r = b

	typeID, ok := nameToTypeID[reflect.TypeOf(o).String()]
	if !ok {
		e = fmt.Errorf("unregister type:%s", reflect.TypeOf(o).String())
	}

	msg := o.(proto.Message)

	data, err := proto.Marshal(msg)
	if err != nil {
		e = err
		return
	}

	dataLen := len(data)
	if dataLen > maxMsgSize {
		e = fmt.Errorf("message size limite maxMsgSize[%d],msg payload[%d]", maxMsgSize, dataLen)
		return
	}

	totalLen := PBHeaderSize + pbIdSize + dataLen

	if nil == b {
		b = buffer.New(make([]byte, 0, totalLen))
		r = b
	}

	//写payload大小
	b.AppendUint32(uint32(totalLen - PBHeaderSize))
	//写类型ID
	b.AppendUint32(typeID)
	//写数据
	b.AppendBytes(data)
	return
}

func Decode(buff []byte, start int, end int, maxMsgSize int) (proto.Message, int, error) {

	dataLen := end - start

	if dataLen < PBHeaderSize {
		return nil, 0, nil
	}

	reader := buffer.NewReader(buff[start:end]) //kendynet.NewByteBuffer(buff[start:end], dataLen)

	payload := reader.GetUint32()

	if int(payload) > maxMsgSize {
		return nil, 0, fmt.Errorf("Decode size limited maxMsgSize[%d],msg payload[%d]", maxMsgSize, payload)
	} else if payload == 0 {
		return nil, 0, fmt.Errorf("Decode header payload == 0")
	}

	totalPacketSize := int(payload) + PBHeaderSize

	if totalPacketSize > dataLen {
		return nil, totalPacketSize, nil
	}

	typeID := reader.GetUint32()

	msg, err := newMessage(typeID)

	if err != nil {
		return nil, totalPacketSize, fmt.Errorf("unregister type:%d", typeID)
	}

	pbDataLen := totalPacketSize - PBHeaderSize - pbIdSize

	pbData := reader.GetBytes(pbDataLen)

	err = proto.Unmarshal(pbData, msg)

	if err != nil {
		return nil, totalPacketSize, err
	}

	return msg, totalPacketSize, nil

}
