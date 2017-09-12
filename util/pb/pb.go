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


var pbMeta map[string]reflect.Type = make(map[string]reflect.Type)

//根据name初始化结构
//在这里根据结构的成员注解进行DI注入，这里没有实现，只是简单都初始化
func New(name string) (c interface{},err error){
   if v,ok := pbMeta[name];ok{
          c = reflect.New(v).Interface()
   } else{
          err = fmt.Errorf("not found %s struct",name)
          //fmt.Printf("not found %s struct\n",name)
   }
   return
}

//根据名字注册实例
func Register(c interface{}) (err error) {

	tt := reflect.TypeOf(c)
	name := reflect.PtrTo(tt).String()

	if _,ok := pbMeta[name];ok {
		err = fmt.Errorf("%s already register",name)
		return
	}

	t := reflect.New(reflect.TypeOf(c)).Interface()

	defer func() {
		recover()
		err = fmt.Errorf("type(%s) not implements proto.Message",reflect.TypeOf(t).String())
	}()

	//检测指针类型是否实现了proto.Message，如果没有会panic,由defer返回错误
	_ = t.(proto.Message)

    pbMeta[name] = tt
    //fmt.Printf("register %s ok\n",name)
    return nil
}

func Encode(o interface{},maxMsgSize uint64) (*kendynet.ByteBuffer,error) {
	msg := o.(proto.Message)
	if msg == nil {
		return nil,fmt.Errorf("msg should be a proto.Message")
	}
	data, err := proto.Marshal(msg)
	if err != nil {
		return nil,err
	}

	dataLen := (uint64)(len(data))
	if dataLen  > maxMsgSize {
		return nil,fmt.Errorf("message size limite")
	}

	msgName := reflect.TypeOf(msg).String()
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
	return buff,nil
}

func Decode(buff []byte,start uint64,end uint64,maxMsgSize uint64) (interface{}/*proto.Message*/,uint64,error) {

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
		return nil,0,fmt.Errorf("Decode size limited")
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

	msg,err := New(msgName)

	if err != nil {
		return nil,0,fmt.Errorf("Decode invaild message:%s",msgName)
	}

	pbDataLen := totalPacketSize - PBHeaderSize - PBStringLenSize - (uint64)(msgNameLen)

	pbData,_ := reader.GetBytes(s,pbDataLen)

	err = proto.Unmarshal(pbData, msg.(proto.Message))

	if err != nil {
		return nil,0,err
	}

	return msg.(proto.Message),totalPacketSize,nil

} 