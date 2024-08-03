package restapi

import (
	"os"

	"github.com/danielpaulus/go-ios/agent/devicestatemgmt"
	"github.com/danielpaulus/go-ios/agent/restapi/api"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// @title           Go-iOS API
// @version         0.01
// @description     Exposes go-ios features as REST API calls.
// @termsOfService  https://github.com/danielpaulus/go-ios

// @contact.name   Daniel Paulus
// @contact.url    https://github.com/danielpaulus/go-ios

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @host      localhost:8080
// @BasePath  /api/v1

// @securityDefinitions.basic  BasicAuth
func StartApi(list *devicestatemgmt.DeviceList) {
	gin.SetMode(gin.ReleaseMode)
	log.WithFields(log.Fields{"args": os.Args, "version": api.GetVersion()}).Infof("starting go-iOS-API")
	api.Main(list)

}
