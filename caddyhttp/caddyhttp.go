// Copyright 2015 Light Code Labs, LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package caddyhttp

import (
	// plug in the server
	_ "github.com/journeymidnight/yig-front-caddy/caddyhttp/httpserver"

	// plug in the standard directives
	_ "github.com/journeymidnight/yig-front-caddy/caddyhttp/basicauth"
	_ "github.com/journeymidnight/yig-front-caddy/caddyhttp/bind"
	_ "github.com/journeymidnight/yig-front-caddy/caddyhttp/browse"
	_ "github.com/journeymidnight/yig-front-caddy/caddyhttp/errors"
	_ "github.com/journeymidnight/yig-front-caddy/caddyhttp/expvar"
	_ "github.com/journeymidnight/yig-front-caddy/caddyhttp/extensions"
	_ "github.com/journeymidnight/yig-front-caddy/caddyhttp/fastcgi"
	_ "github.com/journeymidnight/yig-front-caddy/caddyhttp/gzip"
	_ "github.com/journeymidnight/yig-front-caddy/caddyhttp/header"
	_ "github.com/journeymidnight/yig-front-caddy/caddyhttp/index"
	_ "github.com/journeymidnight/yig-front-caddy/caddyhttp/internalsrv"
	_ "github.com/journeymidnight/yig-front-caddy/caddyhttp/limits"
	_ "github.com/journeymidnight/yig-front-caddy/caddyhttp/log"
	_ "github.com/journeymidnight/yig-front-caddy/caddyhttp/markdown"
	_ "github.com/journeymidnight/yig-front-caddy/caddyhttp/mime"
	_ "github.com/journeymidnight/yig-front-caddy/caddyhttp/pprof"
	_ "github.com/journeymidnight/yig-front-caddy/caddyhttp/prometheus"
	_ "github.com/journeymidnight/yig-front-caddy/caddyhttp/proxy"
	_ "github.com/journeymidnight/yig-front-caddy/caddyhttp/push"
	_ "github.com/journeymidnight/yig-front-caddy/caddyhttp/redirect"
	_ "github.com/journeymidnight/yig-front-caddy/caddyhttp/requestid"
	_ "github.com/journeymidnight/yig-front-caddy/caddyhttp/rewrite"
	_ "github.com/journeymidnight/yig-front-caddy/caddyhttp/root"
	_ "github.com/journeymidnight/yig-front-caddy/caddyhttp/status"
	_ "github.com/journeymidnight/yig-front-caddy/caddyhttp/templates"
	_ "github.com/journeymidnight/yig-front-caddy/caddyhttp/timeouts"
	_ "github.com/journeymidnight/yig-front-caddy/caddyhttp/websocket"
	_ "github.com/journeymidnight/yig-front-caddy/caddyhttp/yig"
	_ "github.com/journeymidnight/yig-front-caddy/onevent"
)
