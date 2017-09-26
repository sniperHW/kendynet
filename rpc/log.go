package rpc

import(
	"github.com/sniperHW/kendynet"
	"github.com/davyxu/golog"	
)

var Logger *golog.Logger

func init() {
	Logger = kendynet.NewLog("rpc")
	Logger.Infoln("rpc log init")
}