package pb

import (
	"github.com/golang/protobuf/proto"
	"github.com/sniperHW/kendynet"
	"fmt"
	"reflect"

)

const (
	PBHeaderSize uint64 = 4
	PBStringLenSize uint64 = 2
)

func Encode(o interface{},maxMsgSize uint64) (r *kendynet.ByteBuffer,e error) {

	msg := o.(proto.Message)

	data, err := proto.Marshal(msg)
	if err != nil {
		e = err
		return
	}

	dataLen := (uint64)(len(data))
	if dataLen  > maxMsgSize {
		e = fmt.Errorf("message size limite maxMsgSize[%d],msg payload[%d]",maxMsgSize,dataLen)
		return
	}

	msgName := proto.MessageName(msg)
	msgNameLen := (uint64)(len(msgName))
	totalLen := PBHeaderSize + PBStringLenSize + msgNameLen + dataLen

	buff := kendynet.NewByteBuffer(totalLen)
	//写payload大小
	buff.AppendUint32((uint32)(totalLen - PBHeaderSize))
	//写类型名大小
	buff.AppendUint16((uint16)(msgNameLen))
	//写类型名
	buff.AppendString(msgName)
	//写数据
	buff.AppendBytes(data)
	r = buff
	return
}

func newMessage(name string) (msg proto.Message,err error){
	mt := proto.MessageType(name)
	if mt == nil {
		err = fmt.Errorf("not found %s struct",name)
	}else {
		msg = reflect.New(mt.Elem()).Interface().(proto.Message)
	}
	return
}

func Decode(buff []byte,start uint64,end uint64,maxMsgSize uint64) (proto.Message,uint64,error) {

	dataLen := end - start

	if dataLen < PBHeaderSize {
		return nil,0,nil
	}

	reader := kendynet.NewByteBuffer(buff[start:end],dataLen)

	s := (uint64)(0)

	size,err := reader.GetUint32(0)

	if err != nil {
		return nil,0,nil
	}

	if (uint64)(size) > maxMsgSize {
		return nil,0,fmt.Errorf("Decode size limited maxMsgSize[%d],msg payload[%d]",maxMsgSize,size)
	}else if (uint64)(size) == 0 {
		return nil,0,fmt.Errorf("Decode header size == 0")
	}

	totalPacketSize := (uint64)(size) + PBHeaderSize

	if totalPacketSize > dataLen {
		return nil,0,nil
	}

	s += PBHeaderSize	

	msgNameLen,_ := reader.GetUint16(s)

	if (uint64)(msgNameLen) > (uint64)(size) - PBStringLenSize {
		return nil,0,fmt.Errorf("Decode invaild message")
	}

	s += PBStringLenSize

	msgName,_ := reader.GetString(s,(uint64)(msgNameLen))

	s += (uint64)(msgNameLen)

	msg,err := newMessage(msgName)

	if err != nil {
		return nil,0,fmt.Errorf("Decode invaild message:%s",msgName)
	}

	pbDataLen := totalPacketSize - PBHeaderSize - PBStringLenSize - (uint64)(msgNameLen)

	pbData,_ := reader.GetBytes(s,pbDataLen)

	err = proto.Unmarshal(pbData, msg)

	if err != nil {
		return nil,0,err
	}

	return msg,totalPacketSize,nil

} 