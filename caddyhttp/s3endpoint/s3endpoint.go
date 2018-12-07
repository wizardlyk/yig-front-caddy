package s3endpoint

import (
	"github.com/journeymidnight/yig-front-caddy"
	"github.com/journeymidnight/yig-front-caddy/caddyhttp/httpserver"
)

func init() {
	caddy.RegisterPlugin("s3endpoint", caddy.Plugin{
		ServerType: "s3endpoint",
		Action:     setupS3Endpoint,
	})
}

func setupS3Endpoint(c *caddy.Controller) error {
	config := httpserver.GetConfig(c)

	for c.Next() {
		if !c.NextArg() {
			return c.ArgErr()
		}
		config.S3Endpoint = c.Val()
		if c.NextArg() {
			// only one argument allowed
			return c.ArgErr()
		}
	}
	return nil
}
