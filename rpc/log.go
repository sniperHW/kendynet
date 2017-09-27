package rpc

import(
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/golog"
)

var Logger *golog.Logger

func init() {
	Logger = kendynet.NewLog("rpc")
	Logger.Infoln("rpc log init")
}