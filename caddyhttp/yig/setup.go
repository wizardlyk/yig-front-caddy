package yig

import (
	"fmt"
	"github.com/journeymidnight/yig-front-caddy"
)

//setup configures a new yig middleware.
func setup(c *caddy.Controller) error {
	//result := yigParse(c)
	//fmt.Println("result", result)
	yigParse(c)
	return nil
}

func yigParse(c *caddy.Controller) (int) {
	for c.Next() {
		//if !c.NextArg() {
		//	return c.ArgErr()
		//}
		//value := c.Val()
		//fmt.Println(value)
		//args := c.RemainingArgs()
		//fmt.Println(len(args))
		for c.NextBlock() {
			what := c.Val()
			where := c.RemainingArgs()
			fmt.Println("what:", what, "--where:", where)
		}
	}
	return 0
}
