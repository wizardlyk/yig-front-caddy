package yig

import (
	"github.com/journeymidnight/yig-front-caddy"
	"net/http"
)

func init() {
	caddy.RegisterPlugin("yig", caddy.Plugin{
		ServerType: "http",
		Action:     setup,
	})
}

type yigHandler struct {

}

func (y yigHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) int{

	return 0
}